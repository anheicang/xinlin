package oss

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	ossSDK "github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type assumeFunc func(*sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error)

func (f assumeFunc) AssumeRole(req *sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error) {
	return f(req)
}

type objectTransport func(*http.Request) (*http.Response, error)

func (f objectTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func stsResponse(expiration string) *sts.AssumeRoleResponse {
	return &sts.AssumeRoleResponse{Body: &sts.AssumeRoleResponseBody{Credentials: &sts.AssumeRoleResponseBodyCredentials{
		AccessKeyId: tea.String("temporary-id"), AccessKeySecret: tea.String("temporary-secret"), SecurityToken: tea.String("temporary-token"), Expiration: tea.String(expiration),
	}}}
}

func mockSTS(t *testing.T, f assumeFunc) {
	t.Helper()
	previous := newSTSClient
	newSTSClient = func(config *openapi.Config) (stsAssumer, error) {
		if tea.StringValue(config.Endpoint) != "sts.cn-hangzhou.aliyuncs.com" {
			t.Error("invalid STS endpoint")
		}
		return f, nil
	}
	t.Cleanup(func() { newSTSClient = previous })
}

func TestSTSRefreshCacheAndConcurrentAccess(t *testing.T) {
	var calls int32
	mockSTS(t, func(req *sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error) {
		atomic.AddInt32(&calls, 1)
		if tea.Int64Value(req.DurationSeconds) != 3600 || tea.StringValue(req.RoleSessionName) != "session" {
			t.Error("wrong STS request")
		}
		return stsResponse(time.Now().Add(time.Hour).UTC().Format(time.RFC3339)), nil
	})
	s := NewSTSService(AliyunOssConfig{StsEndpoint: "cn-hangzhou.aliyuncs.com"})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := s.GetCredentials("session")
			if err != nil || !c.IsValid() {
				t.Errorf("invalid credentials: %v", err)
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("cache failed: %d refreshes", calls)
	}
	if _, err := s.refreshCredentials("session"); err != nil || calls != 1 {
		t.Fatal("double-check cache failed")
	}
	s.ClearCacheBySession("session")
	if _, err := s.GetCredentials("session"); err != nil || calls != 2 {
		t.Fatal("per-session clear failed")
	}
	s.ClearCache()
	if len(s.credentialsCache) != 0 {
		t.Fatal("cache not cleared")
	}
	for _, c := range []*STSCredentials{{}, {AccessKeyId: "id", AccessKeySecret: "secret", SecurityToken: "token", Expiration: time.Now().Add(time.Minute)}} {
		if c.IsValid() {
			t.Fatal("incomplete/expiring credentials accepted")
		}
	}
}

func TestSTSErrorsAndLegacyAPI(t *testing.T) {
	config := AliyunOssConfig{StsEndpoint: "cn-hangzhou.aliyuncs.com", RoleArn: "test-role"}
	for _, tt := range []struct {
		name      string
		response  *sts.AssumeRoleResponse
		err       error
		wantError bool
	}{
		{"transport", nil, errors.New("upstream failed"), true}, {"empty body", &sts.AssumeRoleResponse{}, nil, true}, {"empty credentials", &sts.AssumeRoleResponse{Body: &sts.AssumeRoleResponseBody{}}, nil, true}, {"invalid expiration", stsResponse("invalid"), nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockSTS(t, func(*sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error) { return tt.response, tt.err })
			_, err := NewSTSService(config).GetCredentials("session")
			if (err != nil) != tt.wantError {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
	if _, err := NewSTSService(AliyunOssConfig{}).GetCredentials("session"); err == nil {
		t.Fatal("missing endpoint accepted")
	}
	mockSTS(t, func(req *sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error) {
		if tea.Int64Value(req.DurationSeconds) != 900 || tea.StringValue(req.RoleArn) != "test-role" {
			t.Error("legacy STS request changed")
		}
		return stsResponse(time.Now().Add(time.Hour).Format(time.RFC3339)), nil
	})
	if _, err := GetOssSts(config, "session"); err != nil {
		t.Fatal(err)
	}
	previous := newSTSClient
	newSTSClient = func(*openapi.Config) (stsAssumer, error) { return nil, errors.New("invalid SDK configuration") }
	if _, err := GetOssSts(config, "session"); err == nil {
		t.Fatal("constructor error hidden")
	}
	if _, err := NewSTSService(config).GetCredentials("session"); err == nil {
		t.Fatal("constructor error hidden")
	}
	newSTSClient = previous
}

func objectFixture(t *testing.T, handle objectTransport) (*Uploader, *AliyunOssClient) {
	t.Helper()
	previous := newOSSClient
	newOSSClient = func(endpoint, key, secret string, options ...ossSDK.ClientOption) (*ossSDK.Client, error) {
		options = append(options, ossSDK.HTTPClient(&http.Client{Transport: handle}))
		return ossSDK.New(endpoint, key, secret, options...)
	}
	t.Cleanup(func() { newOSSClient = previous })
	config := AliyunOssConfig{OssEndpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "test-bucket", AccessKeyId: "test-id", AccessKeySecret: "test-secret"}
	s := NewSTSService(config)
	s.credentialsCache["default-session"] = &STSCredentials{AccessKeyId: "temp-id", AccessKeySecret: "temp-secret", SecurityToken: "temp-token", Expiration: time.Now().Add(time.Hour)}
	s.credentialsCache["custom"] = s.credentialsCache["default-session"]
	factory := NewOSSClientFactoryWithSTS(config, s)
	u := NewUploaderWithFactory(factory)
	client, err := NewAliyunOssClient(config)
	if err != nil {
		t.Fatal(err)
	}
	return u, client
}

func objectResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: http.Header{"X-Oss-Request-Id": []string{"fixture-request"}}, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}

func TestUploaderAndObjectOperations(t *testing.T) {
	var puts, deletes int
	u, legacy := objectFixture(t, func(r *http.Request) (*http.Response, error) {
		switch r.Method {
		case "PUT":
			puts++
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			if !strings.HasSuffix(r.URL.Path, "/") && string(body) != "payload" {
				t.Errorf("upload body=%q", body)
			}
			return objectResponse(200, ""), nil
		case "DELETE":
			deletes++
			return objectResponse(204, ""), nil
		case "HEAD":
			response := objectResponse(200, "")
			response.Header.Set("Content-Length", "7")
			response.Header.Set("ETag", `"etag"`)
			return response, nil
		case "GET":
			if r.URL.Query().Has("bucketInfo") {
				return objectResponse(200, `<BucketInfo><Bucket><Name>test-bucket</Name></Bucket></BucketInfo>`), nil
			}
			return objectResponse(200, "payload"), nil
		}
		t.Errorf("unexpected method %s", r.Method)
		return nil, errors.New("unexpected request")
	})
	for _, upload := range []func() (*UploadResult, error){
		func() (*UploadResult, error) { return u.UploadFile("file.txt", "text/plain", []byte("payload")) },
		func() (*UploadResult, error) {
			return u.UploadFileWithSession("custom", "file.txt", "", []byte("payload"))
		},
		func() (*UploadResult, error) {
			return u.UploadFileWithExpiration("file.txt", "text/plain", []byte("payload"), 1)
		},
	} {
		got, err := upload()
		if err != nil || got.Key != "file.txt" || got.Size != 7 || got.Bucket != "test-bucket" {
			t.Fatalf("upload=%+v err=%v", got, err)
		}
	}
	if exists, err := u.FileExists("file.txt"); err != nil || !exists {
		t.Fatal("object not found", err)
	}
	if err := u.DeleteFile("file.txt"); err != nil {
		t.Fatal(err)
	}
	link, err := u.GetSignedURL("file.txt", 60)
	parsed, parseErr := url.Parse(link)
	if err != nil || parseErr != nil || parsed.Query().Get("Signature") == "" {
		t.Fatalf("signed URL=%q err=%v", link, err)
	}
	if u.GetFactory() != u.factory || u.factory.GetSTSService() != u.factory.stsService || u.factory.GetConfig().Bucket != "test-bucket" {
		t.Fatal("factory configuration lost")
	}
	if _, err := u.factory.GetClientWithDefaultSession(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.BuildPath("module", "42", "file.txt"), "/42/") || !strings.HasSuffix(u.BuildPathWithTimestamp("module", "42", ".txt"), ".txt") {
		t.Fatal("incorrect generated key")
	}
	if u.GetFileURL("/file.txt") != "https://test-bucket.oss-cn-hangzhou.aliyuncs.com/file.txt" {
		t.Fatal("incorrect default URL")
	}
	u.config.Domain = "https://cdn.example.com/"
	if u.GetFileURL("/file.txt") != "https://cdn.example.com/file.txt" {
		t.Fatal("incorrect CDN URL")
	}
	if _, err := legacy.GetBucket("test-bucket"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.CreateBucket("test-bucket"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.CreateDir("test-bucket", "dir"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.CreateDir("test-bucket", "dir/"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.UpdateFileByBytes("test-bucket", "file.xlsx", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := legacy.UpdateFileByBytesWithExpiration("test-bucket", "file.xlsx", []byte("payload"), 1); err != nil {
		t.Fatal(err)
	}
	if err := legacy.UploadContent("test-bucket", "file.txt", "text/plain", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if exists, err := legacy.FileExist("test-bucket", "file.txt"); err != nil || !exists {
		t.Fatal("legacy object not found", err)
	}
	if size, etag, err := legacy.GetObjectMeta("test-bucket", "file.txt"); err != nil || size != 7 || etag != "etag" {
		t.Fatalf("metadata=%d/%s err=%v", size, etag, err)
	}
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := legacy.DownloadToFile("test-bucket", "file.txt", file); err != nil {
		t.Fatal(err)
	}
	if value, err := os.ReadFile(file); err != nil || string(value) != "payload" {
		t.Fatal("download content lost", err)
	}
	if err := legacy.UpdateFileByPath("test-bucket", file, "file.txt"); err != nil {
		t.Fatal(err)
	}
	if puts != 9 || deletes != 1 {
		t.Fatalf("puts=%d deletes=%d", puts, deletes)
	}
}

func TestObjectFailuresAndInvalidConfiguration(t *testing.T) {
	u, legacy := objectFixture(t, func(*http.Request) (*http.Response, error) {
		return objectResponse(403, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`), nil
	})
	if _, err := u.UploadFile("file", "", nil); err == nil {
		t.Fatal("upload error hidden")
	}
	if _, err := u.UploadFileWithSession("custom", "file", "", nil); err == nil {
		t.Fatal("upload error hidden")
	}
	if _, err := u.UploadFileWithExpiration("file", "", nil, 1); err == nil {
		t.Fatal("upload error hidden")
	}
	if _, err := u.FileExists("file"); err == nil {
		t.Fatal("existence error hidden")
	}
	if err := u.DeleteFile("file"); err == nil {
		t.Fatal("delete error hidden")
	}
	if _, err := legacy.GetBucket("test-bucket"); err == nil {
		t.Fatal("bucket error hidden")
	}
	if _, _, err := legacy.GetObjectMeta("test-bucket", "file"); err == nil {
		t.Fatal("metadata error hidden")
	}
	if err := legacy.CreateBucket("test-bucket"); err == nil {
		t.Fatal("access denial hidden")
	}
	if err := legacy.UpdateFileByPath("test-bucket", filepath.Join(t.TempDir(), "missing"), "file"); err == nil {
		t.Fatal("read failure hidden")
	}
	for _, name := range []string{"", "UPPERCASE"} {
		if legacy.CreateDir(name, "dir") == nil || legacy.UpdateFileByBytes(name, "file", nil) == nil || legacy.UpdateFileByBytesWithExpiration(name, "file", nil, 1) == nil || legacy.UploadContent(name, "file", "", nil) == nil || legacy.DownloadToFile(name, "file", "unused") == nil {
			t.Fatal("invalid bucket accepted")
		}
		if _, err := legacy.FileExist(name, "file"); err == nil {
			t.Fatal("invalid bucket accepted")
		}
		if _, _, err := legacy.GetObjectMeta(name, "file"); err == nil {
			t.Fatal("invalid bucket accepted")
		}
	}
	broken := NewUploader(AliyunOssConfig{})
	if _, err := broken.UploadFile("file", "", nil); err == nil {
		t.Fatal("invalid factory accepted")
	}
	if _, err := broken.UploadFileWithSession("session", "file", "", nil); err == nil {
		t.Fatal("invalid factory accepted")
	}
	if _, err := broken.UploadFileWithExpiration("file", "", nil, 1); err == nil {
		t.Fatal("invalid factory accepted")
	}
	if _, err := broken.FileExists("file"); err == nil {
		t.Fatal("invalid factory accepted")
	}
	if broken.DeleteFile("file") == nil {
		t.Fatal("invalid factory accepted")
	}
	if _, err := broken.GetSignedURL("file", 60); err == nil {
		t.Fatal("invalid factory accepted")
	}
	if _, err := NewAliyunOssClientWithSTS(AliyunOssConfig{}, "session"); err == nil {
		t.Fatal("missing STS configuration accepted")
	}
	if _, err := u.factory.createClientWithCredentials(&STSCredentials{}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewOSSClientFactory(AliyunOssConfig{}).createClientWithCredentials(&STSCredentials{}); err == nil {
		t.Fatal("missing endpoint accepted")
	}
	previous := newOSSClient
	newOSSClient = func(string, string, string, ...ossSDK.ClientOption) (*ossSDK.Client, error) {
		return nil, errors.New("SDK failed")
	}
	if _, err := NewAliyunOssClient(AliyunOssConfig{}); err == nil {
		t.Fatal("constructor error hidden")
	}
	if _, err := u.factory.GetBucket("default-session"); err == nil {
		t.Fatal("factory constructor error hidden")
	}
	newOSSClient = previous
}

func TestSTSClientAndEndpointNormalization(t *testing.T) {
	for _, input := range []string{" https://oss.example.com/ ", "http://oss.example.com/", "oss.example.com"} {
		if (AliyunOssConfig{OssEndpoint: input}).GetOssEndpoint() != "oss.example.com" {
			t.Fatal("endpoint not normalized")
		}
	}
	// Constructing the real SDK client performs no network I/O.
	if _, err := newSTSClient(&openapi.Config{AccessKeyId: tea.String("id"), AccessKeySecret: tea.String("secret"), Endpoint: tea.String("sts.fixture.invalid")}); err != nil {
		t.Fatal(err)
	}
	mockSTS(t, func(*sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error) {
		return stsResponse(time.Now().Add(time.Hour).Format(time.RFC3339)), nil
	})
	if _, err := NewAliyunOssClientWithSTS(AliyunOssConfig{StsEndpoint: "cn-hangzhou.aliyuncs.com", OssEndpoint: "oss.example.com"}, "session"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMissingBucket(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(fmt.Sprint(denied), func(t *testing.T) {
			_, client := objectFixture(t, func(r *http.Request) (*http.Response, error) {
				if r.Method == "GET" {
					return objectResponse(404, `<Error><Code>NoSuchBucket</Code><Message>missing</Message></Error>`), nil
				}
				if denied {
					return objectResponse(403, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`), nil
				}
				return objectResponse(200, ""), nil
			})
			if err := client.CreateBucket("test-bucket"); (err != nil) != denied {
				t.Fatalf("denied=%v err=%v", denied, err)
			}
		})
	}
}

func TestAdditionalObjectErrorBranches(t *testing.T) {
	u, legacy := objectFixture(t, func(*http.Request) (*http.Response, error) {
		response := objectResponse(200, "")
		response.Header.Set("Content-Length", "not-a-number")
		return response, nil
	})
	if _, _, err := legacy.GetObjectMeta("test-bucket", "file"); err == nil {
		t.Fatal("malformed metadata accepted")
	}
	u.factory.config.Bucket = "INVALID"
	if _, err := u.factory.GetBucketWithDefaultSession(); err == nil {
		t.Fatal("invalid bucket accepted")
	}
	config := AliyunOssConfig{StsEndpoint: "cn-hangzhou.aliyuncs.com", OssEndpoint: "oss.example.com"}
	mockSTS(t, func(*sts.AssumeRoleRequest) (*sts.AssumeRoleResponse, error) {
		return nil, errors.New("STS unavailable")
	})
	if _, err := GetOssSts(config, "session"); err == nil {
		t.Fatal("legacy STS failure hidden")
	}
}
