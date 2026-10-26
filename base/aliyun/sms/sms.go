package sms

type SmsMessage struct {
	TemplateCode string
	Content      map[string]interface{}
	Phone        string
}
type SmsHandler interface {
	SendMessage(message *SmsMessage) error
}
