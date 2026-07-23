package domain

import "fmt"

// MemberID はメンバー（クライアントそれぞれ）の識別子。
type MemberID string

// Member は家計簿を共有するメンバーを表すエンティティ。
type Member struct {
	ID   MemberID
	Name string
}

// Couple は家計簿を共有する2人のメンバーを表す。
// このアプリケーションは常にちょうど2アカウントで利用される。
type Couple struct {
	members [2]Member
}

// NewCouple は2人のメンバーから Couple を生成する。
func NewCouple(a, b Member) (Couple, error) {
	if a.ID == "" || b.ID == "" {
		return Couple{}, fmt.Errorf("%w: メンバーIDは必須です", ErrValidation)
	}
	if a.ID == b.ID {
		return Couple{}, fmt.Errorf("%w: メンバーIDが重複しています: %s", ErrValidation, a.ID)
	}
	return Couple{members: [2]Member{a, b}}, nil
}

// Members は2人のメンバーを返す。
func (c Couple) Members() [2]Member { return c.members }

// Contains は指定IDのメンバーが含まれるかを返す。
func (c Couple) Contains(id MemberID) bool {
	return c.members[0].ID == id || c.members[1].ID == id
}

// Get は指定IDのメンバーを返す。
func (c Couple) Get(id MemberID) (Member, bool) {
	for _, m := range c.members {
		if m.ID == id {
			return m, true
		}
	}
	return Member{}, false
}

// Other は指定IDでない方のメンバーを返す。
func (c Couple) Other(id MemberID) (Member, bool) {
	if c.members[0].ID == id {
		return c.members[1], true
	}
	if c.members[1].ID == id {
		return c.members[0], true
	}
	return Member{}, false
}
