package authentication

const (
	PermUserSelfUpdate = "user:self:update"
	PermUserAnyUpdate  = "user:any:update"
	PermUserAnyDelete  = "user:any:delete"
)

func HasPerm(perms []string, perm string) bool {
	for _, item := range perms {
		if item == perm {
			return true
		}
	}
	return false
}
