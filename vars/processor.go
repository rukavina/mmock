package vars

import (
	"io/ioutil"
	"log"
	"regexp"
	"strings"

	"github.com/rukavina/mmock/definition"
)

var varsRegex = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

type Processor struct {
	FillerFactory FillerFactory
}

func (fp Processor) Eval(req *definition.Request, m *definition.Mock) {
	//load body from file
	loadBody(&m.Response)
	requestFiller := fp.FillerFactory.CreateRequestFiller(req, m)
	fakeFiller := fp.FillerFactory.CreateFakeFiller()
	holders := fp.walkAndGet(m.Response)

	vars := requestFiller.Fill(holders)
	fp.mergeVars(vars, fakeFiller.Fill(holders))
	fp.walkAndFill(m, vars)
}

func loadBody(res *definition.Response) {
	if res.Body != "" || res.BodyFileName == "" {
		return
	}
	data, err := ioutil.ReadFile(res.BodyFileName)
	if err != nil {
		log.Fatalf("Error reading body file [%s]: %s", res.BodyFileName, err)
		return
	}
	res.Body = string(data)
	return
}

func (fp Processor) walkAndGet(res definition.Response) []string {

	vars := []string{}
	for _, header := range res.Headers {
		for _, value := range header {
			fp.extractVars(value, &vars)
		}

	}
	for _, value := range res.Cookies {
		fp.extractVars(value, &vars)
	}

	fp.extractVars(res.Body, &vars)
	return vars
}

func (fp Processor) walkAndFill(m *definition.Mock, vars map[string]string) {
	res := &m.Response
	for header, values := range res.Headers {
		for i, value := range values {
			res.Headers[header][i] = fp.replaceVars(value, vars)
		}

	}
	for cookie, value := range res.Cookies {
		res.Cookies[cookie] = fp.replaceVars(value, vars)
	}

	res.Body = fp.replaceVars(res.Body, vars)
}

func (fp Processor) replaceVars(input string, vars map[string]string) string {
	return varsRegex.ReplaceAllStringFunc(input, func(value string) string {
		varName := strings.Trim(value, "{} ")
		// replace the strings
		if r, found := vars[varName]; found {
			return r
		}
		// replace regexes
		return value
	})
}

func (fp Processor) extractVars(input string, vars *[]string) {
	if m := varsRegex.FindAllString(input, -1); m != nil {
		for _, v := range m {
			varName := strings.Trim(v, "{} ")
			*vars = append(*vars, varName)
		}
	}
}

func (fp Processor) mergeVars(org map[string]string, vals map[string]string) {
	for k, v := range vals {
		org[k] = v
	}
}
