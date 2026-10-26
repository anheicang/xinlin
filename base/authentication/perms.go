// 文件用途：保留通用权限码示例，并提供权限码集合的精确匹配函数。

package authentication

// 这些权限码为旧项目兼容保留，不会由本库自动授予；业务项目可自行定义其他权限码。
const (
	PermUserSelfUpdate = "user:self:update"
	PermUserAnyUpdate  = "user:any:update"
	PermUserAnyDelete  = "user:any:delete"
)

// HasPerm 判断权限列表是否精确包含指定权限码，不进行通配符或管理员推断。
func HasPerm(perms []string, perm string) bool {
	for _, item := range perms {
		if item == perm {
			return true
		}
	}
	return false
}
