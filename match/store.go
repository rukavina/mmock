package match

import (
	"github.com/rukavina/mmock/definition"
)

type Store interface {
	Save(definition.Match)
	Reset()
	GetAll() []definition.Match
}
