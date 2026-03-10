package authz

import (
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"meshcart/gateway/config"
)

const (
	RoleGuest = "guest"
	RoleUser  = "user"
	RoleAdmin = "admin"
)

const (
	ActionReadOnline  = "read_online"
	ActionReadPrivate = "read_private"
	ActionWriteOwn    = "write_own"
	ActionCreate      = "create"
)

type AccessController struct {
	enforcer     *casbin.Enforcer
	adminUserIDs map[int64]struct{}
}

func NewAccessController(cfg config.AdminConfig) (*AccessController, error) {
	m, err := model.NewModelFromString(strings.TrimSpace(`
[request_definition]
r = sub, obj, act, owner, uid, status

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act && ((r.act == "read_online" && r.status == 2) || (r.act == "read_private" && r.owner == r.uid) || (r.act == "write_own" && r.owner == r.uid) || r.act == "create")
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
	}
	for _, policy := range policies {
		_, _ = e.AddPolicy(policy)
	}

	adminUserIDs := make(map[int64]struct{}, len(cfg.UserIDs))
	for _, id := range cfg.UserIDs {
		adminUserIDs[id] = struct{}{}
	}

	return &AccessController{
		enforcer:     e,
		adminUserIDs: adminUserIDs,
	}, nil
}

func (a *AccessController) RoleForUser(userID int64) string {
	if a == nil {
		return RoleUser
	}
	if _, ok := a.adminUserIDs[userID]; ok {
		return RoleAdmin
	}
	return RoleUser
}

func (a *AccessController) IsAdmin(userID int64) bool {
	if a == nil {
		return false
	}
	_, ok := a.adminUserIDs[userID]
	return ok
}

func (a *AccessController) Enforce(role, object, action string, ownerID, userID int64, status int32) bool {
	if a == nil || a.enforcer == nil {
		return false
	}
	ok, err := a.enforcer.Enforce(role, object, action, ownerID, userID, status)
	return err == nil && ok
}
