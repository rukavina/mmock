package match

import (
	"github.com/rukavina/mmock/definition"
)

type Spier interface {
	Find(definition.Request) []definition.Match
	GetMatched() []definition.Match
	GetUnMatched() []definition.Match
	Store
}
