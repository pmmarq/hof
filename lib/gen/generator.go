package gen

import (
	"fmt"
	"time"

	"cuelang.org/go/cue"
)

// A map to generators
type Generators map[string]*Generator

// A generator pulled from the cue instances
type Generator struct {
	//
	// Set by Hof via cuelang extraction
	// Label in Cuelang
	Name string

  // "Global" input, merged with out replacing onto the files
	In map[string]interface{}

  // The list fo files for hof to generate, in cue values
	Out []map[string]interface{}

	//
	// Generator configuration set in Cue code
	//

  // Subgenerators for composition
  Generators []*Generator

  // The following will be automatically added to the template context
  // under its name for reference in GenFiles  and partials in templates
  NamedTemplates map[string]string
  NamedPartials  map[string]string
  // Static files are available for pure cue generators that want to have static files
  // These should be named by their filepath, but be the content of the file
  StaticFiles map[string]string

  //
  // For file based generators
  //   files here will be automatically added to the template context
  //   under its filepath for reference in GenFiles and partials in templates

  // Used for indexing into the vendor directory...
  PackageName string

  // Base directory of entrypoint templates to load
  TemplatesDir string

  // Base directory of partial templatess to load
  PartialsDir string

  // Filepath globs for static files to load
  StaticGlobs []string


	//
	// Hof internal usage
	//

	// Files and the shadow dir for doing neat things
	Files map[string]*File
	Shadow map[string]*File

	// Status for this generator and processing
	Stats *GeneratorStats

	// Cuelang related, also set externally
	CueValue         cue.Value
}

func NewGenerator(label string, value cue.Value) *Generator{
	return &Generator {
		Name: label,
		CueValue: value,
		Files: make(map[string]*File),
		Shadow: make(map[string]*File),
		Stats: &GeneratorStats{},
	}
}

func (G *Generator) GenerateFiles() error {
	errs := []error{}

	start := time.Now()

	// Todo, make this a parallel work queue
	for _, F := range G.Files {
		if F.Filepath == "" {
			continue
		}
		F.ShadowFile = G.Shadow[F.Filepath]
		err := F.Render()
		if err != nil {
			errs = append(errs, fmt.Errorf("In file %q, error %w", F.Filepath, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("Errors while rendering files:\n%v\n", errs)
	}

	elapsed := time.Now().Sub(start).Round(time.Millisecond)
	G.Stats.RenderingTime = elapsed

	return nil
}
