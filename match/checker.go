package match

import (
	"github.com/rukavina/mmock/definition"
)

//Checker checks if the received request matches with some specific mock request definition.
type Checker interface {
	Check(req *definition.Request, mock *definition.Mock, scenarioAware bool) (bool, error)
}
