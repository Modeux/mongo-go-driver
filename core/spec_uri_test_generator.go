// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"path"

	"strings"

	"gopkg.in/yaml.v2"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix(name + ": ")

	var g Generator

	g.printlnf("// Code generated by \"%s.go\"; DO NOT EDIT\n", name)

	src := g.generate()

	err := ioutil.WriteFile(fmt.Sprintf("%s.go", strings.TrimSuffix(name, "_generator")), src, 0644)
	if err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

// Generator holds the state of the analysis. Primarily used to buffer
// the output for format.Source.
type Generator struct {
	buf bytes.Buffer // Accumulated output.
}

// format returns the gofmt-ed contents of the Generator's buffer.
func (g *Generator) format() []byte {
	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		log.Printf("warning: internal error: invalid Go generated: %s", err)
		log.Printf("warning: compile the package to analyze the error")
		return g.buf.Bytes()
	}
	return src
}

func (g *Generator) printf(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, format, args...)
}

func (g *Generator) printlnf(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, format+"\n", args...)
}

func (g *Generator) printIfNotEqual(name string, expected interface{}) {
	g.printlnf(`if %s != %s {`, name, expected)
	g.printlnf(`t.Fatalf("expected %s to be %s, but got \"%%s\"", %s)`,
		strings.Replace(name, "\"", "\\\"", -1),
		strings.Replace(fmt.Sprintf("%v", expected), "\"", "\\\"", -1),
		name)
	g.printlnf("}")
}

func (g *Generator) printStringIfNotEqual(name string, expected string) {
	g.printIfNotEqual(name, fmt.Sprintf("\"%s\"", expected))
}

func (g *Generator) replaceCharacters(target string, old, new string) string {
	j := 0
	for i := 0; i < len(old); i++ {
		target = strings.Replace(target, string(old[i]), string(new[j]), -1)
		if j < len(new)-1 {
			j++
		}
	}

	return target
}

func (g *Generator) replaceNullCharacter(target string) string {
	return strings.Replace(target, "\x00", "\\x00", -1)
}

// EVERYTHING ABOVE IS CONSTANT BETWEEN THE GENERATORS

const name = "spec_uri_test_generator"

func (g *Generator) generate() []byte {
	g.printlnf("package core_test")
	g.printlnf("import \"testing\"")
	g.printlnf("import \"time\"")
	g.printlnf("import . \"github.com/10gen/mongo-go-driver/core\"")

	testsDir := "../specifications/source/connection-string/tests/"

	entries, err := ioutil.ReadDir(testsDir)
	if err != nil {
		log.Fatalf("error reading directory %q: %s", testsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".yml" {
			continue
		}

		g.generateFromFile(path.Join(testsDir, entry.Name()))
	}

	return g.format()
}

func (g *Generator) generateFromFile(filename string) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("error reading file %q: %s", filename, err)
	}

	var testContainer testContainer
	err = yaml.Unmarshal(content, &testContainer)
	if err != nil {
		log.Fatalf("error unmarshalling file %q: %s", filename, err)
	}

	for _, testDef := range testContainer.Tests {
		g.printf("\n\n")
		g.printlnf("func TestParseURI_%s(t *testing.T) {", g.replaceCharacters(testDef.Description, " '-,()", "_"))

		// Validity
		if !testDef.Valid {
			g.printlnf("_, err := ParseURI(%q)", testDef.URI)
			g.printlnf("if err == nil {")
			g.printlnf("t.Fatal(\"expected an error but didn't get one\")")
			g.printlnf("}")
			g.printlnf("}")
			continue
		}

		g.printlnf("uri, err := ParseURI(%q)", testDef.URI)
		g.printlnf("if err != nil {")
		g.printlnf(`t.Fatalf("error parsing \"%%s\": %%s", "%s",  err)`, testDef.URI)
		g.printlnf("}")

		// Hosts
		g.printlnf("if len(uri.Hosts) != %d {", len(testDef.Hosts))
		g.printlnf(`t.Fatalf("expected %d hosts, but had %%d: %%v", len(uri.Hosts), uri.Hosts)`, len(testDef.Hosts))
		g.printlnf("}")
		for i, host := range testDef.Hosts {
			g.printStringIfNotEqual(fmt.Sprintf("uri.Hosts[%d]", i), host.String())
		}

		// Auth
		if testDef.Auth == nil {
			g.printStringIfNotEqual("uri.Username", "")
			g.printlnf(`if uri.PasswordSet {`)
			g.printlnf(`t.Fatalf("expected password to not be set")`)
			g.printlnf("}")
		} else {
			g.printStringIfNotEqual("uri.Username", g.replaceNullCharacter(testDef.Auth.Username))
			g.printStringIfNotEqual("uri.Password", g.replaceNullCharacter(testDef.Auth.Password))
		}

		// Database
		g.printStringIfNotEqual("uri.Database", g.replaceNullCharacter(testDef.Auth.DB))

		// Options
		if testDef.Options != nil && len(testDef.Options) > 0 {
			if value, ok := testDef.Options["authmechanism"]; ok {
				g.printStringIfNotEqual("uri.AuthMechanism", g.replaceNullCharacter(value.(string)))
			} else {
				g.printStringIfNotEqual("uri.AuthMechanism", "")
			}
			if _, ok := testDef.Options["authmechanismproperties"]; ok {
				m := testDef.Options["authmechanismproperties"].(map[interface{}]interface{})
				for key, value := range m {
					g.printStringIfNotEqual(fmt.Sprintf("uri.AuthMechanismProperties[\"%v\"]", key), g.replaceNullCharacter(fmt.Sprintf("%v", value)))
				}
			}
			if value, ok := testDef.Options["replicaset"]; ok {
				g.printStringIfNotEqual("uri.ReplicaSet", g.replaceNullCharacter(value.(string)))
			} else {
				g.printStringIfNotEqual("uri.ReplicaSet", "")
			}
			if value, ok := testDef.Options["wtimeoutms"]; ok {
				g.printIfNotEqual("uri.WTimeout", fmt.Sprintf("time.Duration(%d) * time.Millisecond", value.(int)))
			}
		}

		g.printlnf("}")
	}
}

type testContainer struct {
	Tests []testDef `yaml:"tests"`
}

type testDef struct {
	Auth        *auth  `yaml:"auth"`
	Description string `yaml:"description"`
	Hosts       []host
	Options     map[string]interface{} `yaml:"options"`
	URI         string                 `yaml:"uri"`
	Valid       bool                   `yaml:"valid"`
	Warning     bool                   `yaml:"warning"`
}

type host struct {
	Type string `yaml:"type"`
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func (h *host) String() string {
	s := h.Host
	if h.Type == "ip_literal" {
		s = "[" + s + "]"
	}
	if h.Port != 0 {
		s += fmt.Sprintf(":%d", h.Port)
	}
	return s
}

type auth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       string `yaml:"db"`
}