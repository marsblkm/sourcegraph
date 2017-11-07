package localstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"
	sourcegraph "sourcegraph.com/sourcegraph/sourcegraph/pkg/api"
)

type orgMembers struct{}

func (*orgMembers) Create(ctx context.Context, orgID int32, userID string) (*sourcegraph.OrgMember, error) {
	m := sourcegraph.OrgMember{
		OrgID:  orgID,
		UserID: userID,
	}
	err := globalDB.QueryRowContext(
		ctx,
		"INSERT INTO org_members(org_id, user_id) VALUES($1, $2) RETURNING id, created_at, updated_at",
		m.OrgID, m.UserID).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Constraint == "org_members_org_id_user_id_key" {
				return nil, errors.New("user is already member of the org")
			}
		}
		return nil, err
	}
	return &m, nil
}

func (m *orgMembers) GetByUserID(ctx context.Context, userID string) ([]*sourcegraph.OrgMember, error) {
	return m.getBySQL(ctx, "WHERE user_id=$1", userID)
}

func (m *orgMembers) GetByOrgIDAndUserID(ctx context.Context, orgID int32, userID string) (*sourcegraph.OrgMember, error) {
	if Mocks.OrgMembers.GetByOrgIDAndUserID != nil {
		return Mocks.OrgMembers.GetByOrgIDAndUserID(ctx, orgID, userID)
	}
	return m.getOneBySQL(ctx, "WHERE org_id=$1 AND user_id=$2 LIMIT 1", orgID, userID)
}

func (*orgMembers) Remove(ctx context.Context, orgID int32, userID string) error {
	_, err := globalDB.ExecContext(ctx, "DELETE FROM org_members WHERE (org_id=$1 AND user_id=$2)", orgID, userID)
	return err
}

// GetByOrgID returns a list of all members of a given organization.
func (*orgMembers) GetByOrgID(ctx context.Context, orgID int32) ([]*sourcegraph.OrgMember, error) {
	org, err := Orgs.GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return OrgMembers.getBySQL(ctx, "INNER JOIN users ON org_members.user_id = users.uid WHERE org_id=$1 ORDER BY upper(users.display_name)", org.ID)
}

// ErrOrgMemberNotFound is the error that is returned when
// a user is not in an org.
type ErrOrgMemberNotFound struct {
	args []interface{}
}

func (err ErrOrgMemberNotFound) Error() string {
	return fmt.Sprintf("org member not found: %v", err.args)
}

func (m *orgMembers) getOneBySQL(ctx context.Context, query string, args ...interface{}) (*sourcegraph.OrgMember, error) {
	members, err := m.getBySQL(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(members) != 1 {
		return nil, ErrOrgMemberNotFound{args}
	}
	return members[0], nil
}

func (*orgMembers) getBySQL(ctx context.Context, query string, args ...interface{}) ([]*sourcegraph.OrgMember, error) {
	rows, err := globalDB.QueryContext(ctx, "SELECT org_members.id, org_members.org_id, org_members.user_id, org_members.created_at, org_members.updated_at FROM org_members "+query, args...)
	if err != nil {
		return nil, err
	}

	members := []*sourcegraph.OrgMember{}
	defer rows.Close()
	for rows.Next() {
		m := sourcegraph.OrgMember{}
		err := rows.Scan(&m.ID, &m.OrgID, &m.UserID, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}
		members = append(members, &m)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}
