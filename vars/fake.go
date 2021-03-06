package vars

import (
	"errors"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/rukavina/mmock/vars/fakedata"
)

var errMissingParameterValue = errors.New("The requested method needs input parameters which are not supplied!")

//Fake parses the data looking for fake data tags or request data tags
type Fake struct {
	Fake fakedata.DataFaker
}

func (fv Fake) call(data reflect.Value, name string) (string, error) {
	// get a reflect.Value for the method
	methodVal := data.MethodByName(name)
	// turn that into an interface{}
	methodIface := methodVal.Interface()

	typeOfFunction := reflect.TypeOf(methodIface)
	inputParamsCount := typeOfFunction.NumIn()
	// check whether the method has no input parameters
	if inputParamsCount > 0 {
		return "", errMissingParameterValue
	}

	// turn that into a function that has the expected signature
	method := methodIface.(func() string)

	// call the method directly
	res := method()
	return res, nil
}

func (fv Fake) callWithIntParameter(data reflect.Value, name string, parameter int) string {
	// get a reflect.Value for the method
	methodVal := data.MethodByName(name)
	// turn that into an interface{}
	methodIface := methodVal.Interface()
	// turn that into a function that has the expected signature
	method := methodIface.(func(int) string)
	// call the method directly
	res := method(parameter)
	return res
}

func (fv Fake) callMethod(name string) (string, bool) {
	method, parameter, hasParameter := fv.getMethodAndParameter(name)
	if hasParameter {
		name = method
	}

	found := false
	data := reflect.ValueOf(fv.Fake)
	typ := data.Type()
	if nMethod := data.Type().NumMethod(); nMethod > 0 {
		for i := 0; i < nMethod; i++ {
			method := typ.Method(i)
			if strings.ToLower(method.Name) == strings.ToLower(name) {
				found = true // we found the name regardless
				// does receiver type match? (pointerness might be off)
				if typ == method.Type.In(0) {
					if hasParameter {
						return fv.callWithIntParameter(data, method.Name, parameter), found
					}

					result, err := fv.call(data, method.Name)
					if err != nil {
						log.Printf(err.Error())
					}
					return result, err == nil
				}
			}
		}
	}
	return "", found
}

func (fv Fake) getMethodAndParameter(input string) (method string, parameter int, success bool) {
	r := regexp.MustCompile(`(?P<method>\w+)\((?P<parameter>.*?)\)`)

	match := r.FindStringSubmatch(input)
	result := make(map[string]string)
	names := r.SubexpNames()
	if len(match) >= len(names) {
		for i, name := range names {
			if i != 0 {
				result[name] = match[i]
			}
		}
	}

	method, success = result["method"]
	if !success {
		return
	}

	parameterString, success := result["parameter"]

	parameter, err := strconv.Atoi(parameterString)
	if err != nil {
		success = false
	}

	return
}

func (fv Fake) Fill(holders []string) map[string]string {

	vars := make(map[string]string)
	for _, tag := range holders {
		found := false
		s := ""
		if i := strings.Index(tag, "fake."); i == 0 {
			s, found = fv.callMethod(tag[5:])
		}

		if found {
			vars[tag] = s
		}

	}
	return vars
}
