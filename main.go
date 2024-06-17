package main

import (
	"bufio"
	"flag"
	"os"
	"regexp"
	"strings"
	"text/template"
)

type DockerFileTemplateData struct {
	GoModuleName string
	GoVersion    string
	Port         int
}

var DockerFileTemplate = `# syntax=docker/dockerfile:1

# Build the application from source
FROM golang:{{.GoVersion}} AS build-stage

WORKDIR /app

COPY . .
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /{{.GoModuleName}}

# Run the tests in the container
RUN go test -v ./...
RUN go install golang.org/x/vuln/cmd/govulncheck@latest
RUN govulncheck ./...

# Deploy the application binary into a lean image
FROM gcr.io/distroless/static-debian12 AS build-release-stage

WORKDIR /

COPY --from=build-stage /{{.GoModuleName}} /{{.GoModuleName}}

{{if gt .Port 0}}EXPOSE {{.Port}}

{{end}}USER nonroot:nonroot

ENTRYPOINT ["/{{.GoModuleName}}"]
`

// getProperty returns a property value as a string, panic otherwise
func getProperty(property string, scanner *bufio.Scanner) string {
	propertyRe := regexp.MustCompile(`^` + property + `\s+(.+)$`)
	for scanner.Scan() {
		matches := propertyRe.FindStringSubmatch(scanner.Text())
		if len(matches) == 2 {
			return matches[1]
		}
	}
	panic("Missing property: " + property)
}

func main() {
	Port := flag.Int("p", 0, "Port to listen on")
	DockerFileName := flag.String("o", "Dockerfile", "Dockerfile name")
	flag.Parse()

	goMod, err := os.Open("go.mod")
	if err != nil {
		panic(err)
	}
	defer goMod.Close()
	scanner := bufio.NewScanner(goMod)

	GoModule := getProperty("module", scanner)
	modulePart := strings.Split(GoModule, "/")
	GoModule = modulePart[len(modulePart)-1]

	GoVersion := getProperty("go", scanner)

	TemplateData := DockerFileTemplateData{
		GoModuleName: GoModule,
		GoVersion:    GoVersion,
		Port:         *Port,
	}

	t, err := template.New("Dockerfile").Parse(DockerFileTemplate)
	if err != nil {
		panic(err)
	}

	var templateBuffer strings.Builder
	if err := t.Execute(&templateBuffer, TemplateData); err != nil {
		panic(err)
	}

	if os.WriteFile(*DockerFileName, []byte(templateBuffer.String()), 0644) != nil {
		panic(err)
	}
}
