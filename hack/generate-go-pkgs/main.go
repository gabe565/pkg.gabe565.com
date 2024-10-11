package main

import (
	_ "embed"
	"errors"
	"flag"
	"html/template"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Host        string    `yaml:"host"`
	DefaultUser string    `yaml:"defaultUser"`
	Packages    []Package `yaml:"packages"`
}

type Package struct {
	Name string `yaml:"name"`
	Repo string `yaml:"repo"`
}

//go:embed config.yaml
var configData []byte

//go:embed template.html.gotmpl
var pkgTemplateStr string

var pkgTemplate = template.Must(template.New("").Parse(pkgTemplateStr))

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("generate-go-pkgs", flag.ExitOnError)

	var gitignore bool
	fs.BoolVar(&gitignore, "gitignore", true, "Generate .gitignore files for each pkg")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return err
	}

	var errs []error
	for _, pkg := range config.Packages {
		if err := templatePkg(config, pkg, gitignore); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func templatePkg(config Config, pkg Package, gitignore bool) error {
	pkgPath := filepath.Join("public", pkg.Name)
	if err := os.MkdirAll(pkgPath, 0o755); err != nil && !os.IsExist(err) {
		return err
	}

	if gitignore {
		if err := os.WriteFile(filepath.Join(pkgPath, ".gitignore"), []byte(".gitignore\nindex.html"), 0o600); err != nil {
			return err
		}
	}

	repo, err := url.Parse(pkg.Repo)
	if err != nil {
		return err
	}
	switch {
	case repo.Path == "":
		repo.Path = path.Join(config.DefaultUser, pkg.Name)
	case !strings.Contains(repo.Path, "/"):
		repo.Path = path.Join(config.DefaultUser, pkg.Repo)
	}
	if repo.Scheme == "" {
		repo.Scheme = "https"
	}
	if repo.Host == "" {
		repo.Host = "github.com"
	}
	pkg.Repo = repo.String()

	f, err := os.Create(filepath.Join(pkgPath, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	slog.Info("Generating HTML", "name", pkg.Name, "repo", pkg.Repo)
	if err := pkgTemplate.Execute(f, map[string]any{
		"Config": config,
		"Pkg":    pkg,
	}); err != nil {
		return err
	}

	return f.Close()
}
