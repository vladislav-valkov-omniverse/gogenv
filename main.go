package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	. "github.com/dave/jennifer/jen"
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

	file := NewFile(t.PackageName)
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

func GenerateImports(file *File, imports []Import) {
	opt := Options{
		Open:      "(",
		Close:     ")",
		Separator: "",
		Multi:     true,
	}

	ids := []Code{}
	for _, v := range imports {
		ids = append(ids, Id(v.Alias+" \""+v.Path+"\""))
	}

	file.Id("import").Custom(opt, ids...)
}

func GenerateInterface(file *File, variables []Variable, t Template) {
	methods := make([]Code, len(variables))
	for _, variable := range variables {

		methods = append(methods, Id(variable.Name).Params().Id(variable.Type))
	}

	file.Type().Id("I" + t.ConfigName).Interface(methods...)
}

func GenerateConfigConstructor(file *File, t Template) {
	file.Func().Id("New"+t.ConfigName).Params(Id("path").Id("string")).Params(Id("I"+t.ConfigName), Error()).Block(

		Var().Id("v").Op("=").Id("&appConfig").Block(
			Id("viper").Op(":").Qual("github.com/spf13/viper", "New").Call(),
			Op(","),
		),

		Err().Op(":=").Id("v").Dot("loadViperConfig").Call(Id("path")),
		If(Err().Op("!=").Nil()).Block(
			Return(List(Nil(), Err()))),
		Return(List(Id("v"), Nil())),
	)
}

func GenerateStruct(file *File, variables []Variable, t Template) {
	structFields := make([]Code, len(variables))

	{
		viperField := Id("viper").Op("*").Qual("github.com/spf13/viper", "Viper")
		structFields = append(structFields, viperField)
	}

	for _, variable := range variables {
		field := Id("Field" + variable.Name).Id(variable.Type).Tag(map[string]string{
			"mapstructure": variable.RawName,
		})
		structFields = append(structFields, field)
	}

	file.Type().Id("appConfig").Struct(structFields...)

	// Generate methods
	for _, variable := range variables {
		file.Func().
			Params(Id("this").Id("*appConfig")).Id(variable.Name).
			Params().
			Id(variable.Type).
			Block(Return(Id("this").Dot("Field" + variable.Name)))
	}

	// Generate configLoader
	file.Func().
		Params(Id("this").Id("*appConfig")).Id("loadViperConfig").
		Params(Id("path").String()).
		Params(Id("err").Error()).
		Block(
			Id("this").Dot("viper").Dot("AddConfigPath").Call(Id("path")),
			Id("this").Dot("viper").Dot("SetConfigName").Call(Lit(strings.Split(t.EnvFile, ".")[0])),
			Id("this").Dot("viper").Dot("SetConfigType").Call(Lit(strings.Split(t.EnvFile, ".")[1])),
			Id("this").Dot("viper").Dot("AutomaticEnv").Call(),
			Id("_").Op("=").Qual("github.com/spf13/viper", "ReadInConfig").Call(),
			Id("this").Dot("setDefaults").Call(),
			Id("err").Op("=").Id("viper").Dot("Unmarshal").Call(Id("this")),

			If(Err().Op("!=").Nil()).Block(
				Err().Op("=").Qual("fmt", "Errorf").Call(Lit(fmt.Sprintf("[%s] Failed to load environment", t.ConfigName)))),
			Return(),
		)

	// Generate setDefault

	defaults := make([]Code, len(variables))

	for _, defaultVariable := range variables {
		defaults = append(defaults, Id("this").Dot("viper").Dot("SetDefault").
			Call(List(Lit(defaultVariable.RawName), Lit(defaultVariable.Default))))
	}

	file.Func().
		Params(Id("this").Id("*appConfig")).Id("setDefaults").Params().
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
