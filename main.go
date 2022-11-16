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
	EnvFile   string     `yaml:"envFile"`
	Variables []Variable `yaml:"variables"`
}

type Variable struct {
	Name    string `yaml:"name"`
	RawName string
	Type    string `yaml:"type"`
	Default string `yaml:"default"`
}

func main() {

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

	file := jen.NewFile("appconfig")
	file.HeaderComment(`//go:generate echo "Running Gogenv`)
	file.HeaderComment(`//go:generate gogenv`)
	file.Comment("This file is generated automatically.")
	file.Line()

	t.Variables = FormatVariables(t.Variables)

	GenerateConfigConstructor(file)
	GenerateInterface(file, t.Variables)
	GenerateStruct(file, t.Variables, t.EnvFile)

	writeF, _ := os.Create("appconfig_generated.go")
	_, err = writeF.Write([]byte(fmt.Sprintf("%#v", file)))
	if err != nil {
		log.Fatal(err)
	}

}

func GenerateInterface(file *jen.File, variables []Variable) {
	methods := make([]jen.Code, len(variables))
	for _, variable := range variables {
		methods = append(methods, jen.Id(variable.Name).Params().Id(variable.Type))
	}

	file.Type().Id("IAppConfig").Interface(methods...)
}

func GenerateConfigConstructor(file *jen.File) {
	file.Func().Id("NewAppConfig").Params(jen.Id("path").Id("string")).Params(jen.Id("IAppConfig"), jen.Error()).Block(
		jen.Var().Id("v").Op("=").Id("&appConfig").Block(),
		jen.Err().Op(":=").Id("v").Dot("loadViperConfig").Call(jen.Id("path")),
		jen.If(jen.Err().Op("!=").Nil()).Block(
			jen.Return(jen.List(jen.Nil(), jen.Err()))),
		jen.Return(jen.List(jen.Id("v"), jen.Nil())),
	)
}

func GenerateStruct(file *jen.File, variables []Variable, envFile string) {
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
			jen.Qual("github.com/spf13/viper", "SetConfigName").Call(jen.Lit(strings.Split(envFile, ".")[0])),
			jen.Qual("github.com/spf13/viper", "SetConfigType").Call(jen.Lit(strings.Split(envFile, ".")[0])),
			jen.Qual("github.com/spf13/viper", "AutomaticEnv").Call(),
			jen.Id("_").Op("=").Qual("github.com/spf13/viper", "ReadInConfig").Call(),
			jen.Id("this").Dot("setDefaults").Call(),
			jen.Id("err").Op("=").Id("viper").Dot("Unmarshal").Call(jen.Id("this")),

			jen.If(jen.Err().Op("!=").Nil()).Block(
				jen.Err().Op("=").Qual("fmt", "Errorf").Call(jen.Lit("[AppConfig] Failed to load environment"))),
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
