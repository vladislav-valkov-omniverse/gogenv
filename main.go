package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dave/jennifer/jen"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Template struct {
	EnvFile     string     `yaml:"envFile"`
	PackageName string     `yaml:"packageName"`
	ConfigName  string     `yaml:"configName"`
	Imports     []Import   `yaml:"imports"`
	Variables   []Variable `yaml:"variables"`
}

type Import struct {
	Path  string `yaml:"path"`
	Alias string `yaml:"alias"`
}

type Variable struct {
	Name    string `yaml:"name"`
	RawName string
	Type    string `yaml:"type"`
	Default string `yaml:"default"`
}

func main() {

	fmt.Println("Generating schema for environment...")
	f, err := os.ReadFile("template.yaml")
	var t Template

	if err != nil {
		log.Fatal(err)
	}

	if err := yaml.Unmarshal(f, &t); err != nil {
		log.Fatal(err)
	}

	//cmd := exec.Command("go get", "github.com/spf13/viper")
	//_, err = cmd.Output()
	//if err != nil {
	//	log.Fatal(err)
	//}

	if t.PackageName == "" {
		t.PackageName = "appconfig"
	}
	if t.ConfigName == "" {
		t.ConfigName = "AppConfig"
	}

	file := jen.NewFile(t.PackageName)
	file.HeaderComment("//go:generate gogenv && go fmt .")
	file.HeaderComment("This file is generated automatically.")
	file.Line()

	t.Variables = FormatVariables(t.Variables)

	GenerateImports(file, t.Imports)
	file.Line()
	GenerateConfigConstructor(file, t)
	GenerateInterface(file, t.Variables, t)
	GenerateStruct(file, t.Variables, t)

	writeF, _ := os.Create(strings.ToLower(t.ConfigName) + "_generated.go")
	_, err = writeF.Write([]byte(fmt.Sprintf("%#v", file)))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Schema generated successfully!")
}

func GenerateImports(file *jen.File, imports []Import) {
	opt := jen.Options{
		Open:      "(",
		Close:     ")",
		Separator: "",
		Multi:     true,
	}

	ids := []jen.Code{}
	for _, v := range imports {
		ids = append(ids, jen.Id(v.Alias+" \""+v.Path+"\""))
	}

	file.Id("import").Custom(opt, ids...)
}

func GenerateInterface(file *jen.File, variables []Variable, t Template) {
	methods := make([]jen.Code, len(variables))
	for _, variable := range variables {

		methods = append(methods, jen.Id(variable.Name).Params().Id(variable.Type))
	}

	file.Type().Id("I" + t.ConfigName).Interface(methods...)
}

func GenerateConfigConstructor(file *jen.File, t Template) {
	file.Func().Id("New"+t.ConfigName).Params(jen.Id("path").Id("string")).Params(jen.Id("I"+t.ConfigName), jen.Error()).Block(
		jen.Var().Id("v").Op("=").Id("&appConfig").Block(),
		jen.Err().Op(":=").Id("v").Dot("loadViperConfig").Call(jen.Id("path")),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.List(jen.Nil(), jen.Err()))),
		jen.Return(jen.List(jen.Id("v"), jen.Nil())),
	)
}

func GenerateStruct(file *jen.File, variables []Variable, t Template) {
	structFields := make([]jen.Code, len(variables))

	for _, variable := range variables {
		field := jen.Id("Field" + variable.Name).Id(variable.Type).Tag(map[string]string{
			"mapstructure": variable.RawName,
		})
		structFields = append(structFields, field)
	}

	file.Type().Id("appConfig").Struct(structFields...)

	// Generate methods
	for _, variable := range variables {
		file.Func().
			Params(jen.Id("this").Id("*appConfig")).Id(variable.Name).
			Params().
			Id(variable.Type).
			Block(jen.Return(jen.Id("this").Dot("Field" + variable.Name)))
	}

	// Generate configLoader
	file.Func().
		Params(jen.Id("this").Id("*appConfig")).Id("loadViperConfig").
		Params(jen.Id("path").String()).
		Params(jen.Id("err").Error()).
		Block(
			jen.Qual("github.com/spf13/viper", "AddConfigPath").Call(jen.Id("path")),
			jen.Qual("github.com/spf13/viper", "SetConfigName").Call(jen.Lit(strings.Split(t.EnvFile, ".")[0])),
			jen.Qual("github.com/spf13/viper", "SetConfigType").Call(jen.Lit(strings.Split(t.EnvFile, ".")[1])),
			jen.Qual("github.com/spf13/viper", "AutomaticEnv").Call(),
			jen.Id("_").Op("=").Qual("github.com/spf13/viper", "ReadInConfig").Call(),
			jen.Id("this").Dot("setDefaults").Call(),
			jen.Id("err").Op("=").Id("viper").Dot("Unmarshal").Call(jen.Id("this")),

			jen.If(jen.Err().Op("!=").Nil()).Block(
				jen.Err().Op("=").Qual("fmt", "Errorf").Call(jen.Lit(fmt.Sprintf("[%s] Failed to load environment", t.ConfigName)))),
			jen.Return(),
		)

	// Generate setDefault

	defaults := make([]jen.Code, len(variables))

	for _, defaultVariable := range variables {
		defaults = append(defaults, jen.Qual("github.com/spf13/viper", "SetDefault").
			Call(jen.List(jen.Lit(defaultVariable.RawName), jen.Lit(defaultVariable.Default))))
	}

	file.Func().
		Params(jen.Id("this").Id("*appConfig")).Id("setDefaults").Params().
		Block(defaults...)

}

func FormatVariables(variables []Variable) []Variable {
	for variableIndex, variable := range variables {
		variable.RawName = variable.Name
		splitted := strings.Split(variable.Name, "_")
		for i := range splitted {
			splitted[i] = cases.Title(language.English).String(splitted[i])
		}
		variable.Name = strings.Join(splitted, "")

		variables[variableIndex] = variable

		if strings.HasSuffix(variable.Type, "[]") {
			before, after, _ := strings.Cut(variable.Type, "[]")
			variables[variableIndex].Type = "[]" + before + after
		}

	}

	return variables
}
