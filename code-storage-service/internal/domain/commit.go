package domain

type Commit struct {
	ID             string
	Name           string
	Root_commits   *Commit
	Parent_commits []*Commit
	Code           []byte
}
