package authz

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

const (
	PermViewDevices = "devices:view"
	PermSendCommand = "devices:command"
	PermManageUsers = "users:manage"
	PermViewPSA     = "psa:view"
	PermManagePSA   = "psa:manage"
)

var rolePermissions = map[string]map[string]struct{}{
	RoleAdmin: {
		PermViewDevices: {},
		PermSendCommand: {},
		PermManageUsers: {},
		PermViewPSA:     {},
		PermManagePSA:   {},
	},
	RoleOperator: {
		PermViewDevices: {},
		PermSendCommand: {},
		PermViewPSA:     {},
		PermManagePSA:   {},
	},
	RoleViewer: {
		PermViewDevices: {},
		PermViewPSA:     {},
	},
}

func IsValidRole(role string) bool {
	_, ok := rolePermissions[role]
	return ok
}

func Permissions(role string) []string {
	permsMap, ok := rolePermissions[role]
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(permsMap))
	for p := range permsMap {
		out = append(out, p)
	}
	return out
}

func Can(role, permission string) bool {
	permsMap, ok := rolePermissions[role]
	if !ok {
		return false
	}
	_, exists := permsMap[permission]
	return exists
}
