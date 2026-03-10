package authz

import (
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

const (
	RoleGuest      = "guest"
	RoleUser       = "user"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "superadmin"
)

const (
	ActionReadOnline  = "read_online"
	ActionReadPrivate = "read_private"
	ActionWriteOwn    = "write_own"
	ActionCreate      = "create"
	ActionManageRole  = "manage_role"
)

type AccessController struct {
	enforcer *casbin.Enforcer
}

func NewAccessController() (*AccessController, error) {
	m, err := model.NewModelFromString(strings.TrimSpace(`
[request_definition]
r = sub, obj, act, owner, uid, status

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act && ((r.act == "read_online" && r.status == 2) || (r.act == "read_private" && (r.sub == "superadmin" || r.owner == r.uid)) || (r.act == "write_own" && (r.sub == "superadmin" || r.owner == r.uid)) || r.act == "create" || r.act == "manage_role")
`))
	if err != nil {
		return nil, err
	}

	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}

	policies := [][]string{
		{RoleGuest, "product", ActionReadOnline},
		{RoleUser, "product", ActionReadOnline},
		{RoleAdmin, "product", ActionReadOnline},
		{RoleAdmin, "product", ActionReadPrivate},
		{RoleAdmin, "product", ActionWriteOwn},
		{RoleAdmin, "product", ActionCreate},
		{RoleSuperAdmin, "product", ActionReadOnline},
		{RoleSuperAdmin, "product", ActionReadPrivate},
		{RoleSuperAdmin, "product", ActionWriteOwn},
		{RoleSuperAdmin, "product", ActionCreate},
		{RoleSuperAdmin, "user", ActionManageRole},
	}
	for _, policy := range policies {
		_, _ = e.AddPolicy(policy)
	}

	return &AccessController{
		enforcer: e,
	}, nil
}

func (a *AccessController) Enforce(role, object, action string, ownerID, userID int64, status int32) bool {
	if a == nil || a.enforcer == nil {
		return false
	}
	ok, err := a.enforcer.Enforce(role, object, action, ownerID, userID, status)
	return err == nil && ok
}
