package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/hashicorp/hcl"
)

type Configuration struct {
	Rules []ConfigRule `hcl:"access"`
}

var config = &Configuration{}

// ConfigRule represents a single entry from rules.hcl file
// example:
//     access "allow" {
//        path = "/api"
//     }
// would like to be able to do allow/deny {...} , but I suck at HCL
type ConfigRule struct {
	Mode      string `hcl:",key"`
	Username  string
	Verb      string
	Group     string
	Resource  string
	Namespace string
	Path      string
}

type AccessMode int

const (
	NOMATCH AccessMode = 1 + iota
	ALLOW
	DENY
)

// LoadConfigFromFile loads configuration from
// hcl into config structure
func LoadConfigFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	hclText, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	return LoadConfigFromByteArray(hclText)

}

// LoadConfigFromByteArray loads configuration from
// []byte
func LoadConfigFromByteArray(hclText []byte) error {
	hclParseTree, err := hcl.Parse(string(hclText))
	if err != nil {
		return err
	}

	if err := hcl.DecodeObject(&config, hclParseTree); err != nil {
		return err
	}
	return nil
}

// GetAccessMode returns allow/deny if request is described in the rule
func (s *ConfigRule) GetAccessMode(context *RequestContext) AccessMode {

	matchData := map[string]string{
		s.Username:  context.Username,
		s.Verb:      context.Request.Action(),
		s.Group:     context.Request.Group(),
		s.Resource:  context.Request.Resource(),
		s.Namespace: context.Request.Namespace(),
		s.Path:      context.Request.Path(),
	}

	for k, v := range matchData {
		t, err := compileTemplate(k, context)
		if err != nil {
			fmt.Println(err)
			return NOMATCH
		}
		res := match(t, v)
		if res == false {
			return NOMATCH
		}
	}
	if s.Mode == "deny" {
		return DENY
	}
	return ALLOW
}

func compileTemplate(tmpl string, context *RequestContext) (string, error) {
	fm := template.FuncMap{
		"substring": func(str string, idx int) string {
			return str[:idx]
		},
		"replace": func(str string, regSrc string, regDst string) string {
			re, _ := regexp.Compile(regSrc)
			retval := re.ReplaceAll([]byte(str), []byte(regDst))
			return string(retval)
		},
	}

	t, err := template.New("m").Funcs(fm).Parse(tmpl)
	if err != nil {
		return "", err
	}

	buff := new(bytes.Buffer)
	err = t.Execute(buff, context)
	if err != nil {
		return "", err
	}
	return buff.String(), nil
}

func match(field string, str string) bool {
	if field == "" || field == "*" {
		return true
	}

	compiledField := fmt.Sprintf("^%s$", field)
	// fmt.Printf("compiled field: %v, value: %v\n", compiledField, str)
	match, err := regexp.MatchString(compiledField, str)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return false
	}
	return match
}
