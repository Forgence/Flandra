package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

var extractionFuncs = map[string]func(string, bool, bool, bool, bool, string) (string, error){
	".go": extractGo,
	// ".rs": extractRust,
	// ".cs": extractCSharp,
	// ".py": extractPython,
	// ".sh": extractShellScript,
}

type FileCode struct {
	Filename string
	Code     string
}

func main() {
	// Define flags
	var (
		dir              string
		subDirs          bool
		size             int64
		fileType         string
		modifiedSince    string
		extractFuncs     bool
		extractImports   bool
		extractGlobals   bool
		generateComments bool
		outFile          string
		apiKey           string
	)

	flag.StringVar(&dir, "dir", ".", "Define the directory in which to begin or default to the current directory")
	flag.BoolVar(&subDirs, "subDirs", false, "Allow a flag to go into subDirs or not if looking at the whole dir")
	flag.Int64Var(&size, "size", 0, "Filter based on file size (in bytes), default to no size filter")
	flag.StringVar(&fileType, "type", "", "Filter based on file type, default to no type filter")
	flag.StringVar(&modifiedSince, "modified", "", "Filter based on last modified time, default to no time filter")
	flag.BoolVar(&extractFuncs, "extractFuncs", true, "If set, function declarations will be extracted")
	flag.BoolVar(&extractImports, "extractImports", true, "If set, import statements will be extracted")
	flag.BoolVar(&extractGlobals, "extractGlobals", true, "If set, global variable declarations will be extracted")
	flag.StringVar(&outFile, "out", "output.txt", "Output file to write the combined code, default to output.txt")
	flag.BoolVar(&generateComments, "generateComments", false, "If set, comments will be generated for functions")
	flag.StringVar(&apiKey, "apiKey", "", "OpenAI API key")

	flag.Parse()

	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			fmt.Println("API key not provided. Set it via the -apiKey flag or the OPENAI_API_KEY environment variable.")
			os.Exit(1)
		}
	}

	// Walk the file system
	files, err := walkFileSystem(dir, subDirs, size, fileType, modifiedSince)
	if err != nil {
		fmt.Println("Error walking file system:", err)
		os.Exit(1)
	}

	// Extract code from the files
	codes, err := extractCode(files, extractFuncs, extractImports, extractGlobals, generateComments, apiKey)
	if err != nil {
		fmt.Println("Error extracting code:", err)
		os.Exit(1)
	}

	// Write the code to the output file
	err = writeOutput(codes, outFile, generateComments)
	if err != nil {
		fmt.Println("Error writing output:", err)
		os.Exit(1)
	}

	fmt.Println("Successfully combined code into", outFile)
}

func walkFileSystem(dir string, subDirs bool, size int64, fileType string, modifiedSince string) ([]string, error) {
	// Parse the modifiedSince string into a time.Time
	var modTime time.Time
	var err error
	if modifiedSince != "" {
		modTime, err = time.Parse(time.RFC3339, modifiedSince)
		if err != nil {
			return nil, fmt.Errorf("invalid time format for -modified: %v", err)
		}
	}

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// If subDirs is false and this is a directory other than the starting directory, skip it
		if !subDirs && info.IsDir() && path != dir {
			return filepath.SkipDir
		}
		// Skip if this is a directory
		if info.IsDir() {
			return nil
		}
		// Check file size
		if info.Size() < size {
			return nil
		}
		// Check file type
		if fileType != "" && filepath.Ext(path) != fileType {
			return nil
		}
		// Check last modified time
		if !modTime.IsZero() && info.ModTime().Before(modTime) {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func extractCode(files []string, extractFuncs, extractImports, extractGlobals, generateComments bool, apiKey string) ([]FileCode, error) {
	var codes []FileCode
	for _, file := range files {
		// Read the file
		content, err := readFileContent(file)
		if err != nil {
			return nil, err
		}

		// Extract the code
		code, err := extractCodeFromFile(file, content, extractFuncs, extractImports, extractGlobals, generateComments, apiKey)
		if err != nil {
			return nil, err
		}

		codes = append(codes, FileCode{Filename: file, Code: code})
	}
	return codes, nil
}

func readFileContent(file string) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("error reading file %s: %v", file, err)
	}

	return string(content), nil
}

func extractCodeFromFile(file, content string, extractFuncs, extractImports, extractGlobals, generateComments bool, apiKey string) (string, error) {
	// Declare and initialize buf and f
	var buf strings.Builder
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", content, 0)
	if err != nil {
		return "", fmt.Errorf("error parsing Go code: %v", err)
	}

	// Determine the extraction function based on the file type
	ext := filepath.Ext(file)
	extractionFunc, ok := extractionFuncs[ext]
	if !ok {
		// Skip files with unsupported extensions
		fmt.Printf("Skipping file with unsupported extension: %s\n", file)
		return "", nil
	}

	if extractFuncs {
		extractFuncsFromAst(&buf, f, fset, generateComments, apiKey)
	}

	// Extract the code
	code, err := extractionFunc(content, extractFuncs, extractImports, extractGlobals, generateComments, apiKey)
	if err != nil {
		return "", fmt.Errorf("error extracting code from file %s: %v", file, err)
	}

	return code, nil
}

func extractGo(content string, extractFuncs, extractImports, extractGlobals, generateComments bool, apiKey string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", content, 0)
	if err != nil {
		return "", fmt.Errorf("error parsing Go code: %v", err)
	}

	var buf strings.Builder

	if extractImports {
		extractImportsFromAst(&buf, f)
	}

	if extractGlobals {
		extractGlobalsFromAst(&buf, f, fset)
	}

	if extractFuncs {
		extractFuncsFromAst(&buf, f, fset, generateComments, apiKey)
	}

	return buf.String(), nil
}

func extractImportsFromAst(buf *strings.Builder, f *ast.File) {
	for _, imp := range f.Imports {
		buf.WriteString("import ")
		buf.WriteString(imp.Path.Value)
		buf.WriteString("\n")
	}
}

func extractGlobalsFromAst(buf *strings.Builder, f *ast.File, fset *token.FileSet) {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				buf.WriteString("var ")
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						for _, name := range s.Names {
							buf.WriteString(name.Name)
							buf.WriteString(" ")
						}
						printer.Fprint(buf, fset, s.Type)
						buf.WriteString("\n")
					}
				}
			}
		}
	}
}

func extractFuncsFromAst(buf *strings.Builder, f *ast.File, fset *token.FileSet, generateComments bool, apiKey string) {
	for _, decl := range f.Decls {
		if fn, isFn := decl.(*ast.FuncDecl); isFn {
			buf.WriteString("func ")
			buf.WriteString(fn.Name.Name)
			buf.WriteString(formatParams(fn.Type.Params))
			buf.WriteString(formatResults(fn.Type.Results)) // Add this line to extract return types
			buf.WriteString(" {\n")
			if generateComments {
				comment, err := generateComment(fn.Name.Name+formatParams(fn.Type.Params), apiKey)
				if err != nil {
					fmt.Printf("Error generating comment for function %s: %v\n", fn.Name.Name, err)
				} else {
					buf.WriteString("// " + comment + "\n")
				}
			}
			buf.WriteString("}\n")
		}
	}
}

func formatResults(results *ast.FieldList) string {
	if results == nil {
		return ""
	}
	var buf strings.Builder
	buf.WriteString(" (")
	for i, result := range results.List {
		if i > 0 {
			buf.WriteString(", ")
		}
		var typeBuf bytes.Buffer
		printer.Fprint(&typeBuf, token.NewFileSet(), result.Type)
		buf.WriteString(typeBuf.String())
	}
	buf.WriteString(")")
	return buf.String()
}

func formatParams(params *ast.FieldList) string {
	var buf strings.Builder
	buf.WriteString("(")
	for i, param := range params.List {
		if i > 0 {
			buf.WriteString(", ")
		}
		for j, name := range param.Names {
			if j > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(name.Name)
		}
		buf.WriteString(" ")
		var typeBuf bytes.Buffer
		printer.Fprint(&typeBuf, token.NewFileSet(), param.Type)
		buf.WriteString(typeBuf.String())
	}
	buf.WriteString(")")
	return buf.String()
}

func writeOutput(codes []FileCode, outFile string, generateComments bool) error {
	// Open the output file for writing
	file, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, filecode := range codes {
		// Write the file name and code to the output file
		_, err := writer.WriteString(fmt.Sprintf("'''%s\n%s\n'''\n", filecode.Filename, filecode.Code))
		if err != nil {
			return fmt.Errorf("error writing to output file: %v", err)
		}
	}

	// Make sure everything gets written to the file
	writer.Flush()

	return nil
}

func generateComment(code string, apiKey string) (string, error) {
	c := openai.NewClient(apiKey)

	// c := openai.NewClient("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") // Create a new client and the key is already invalid. :p
	ctx := context.Background()

	// Add a prompt that makes it clear that the model should generate a comment for a function
	prompt := "Generate a descriptive comment for the following Go function:\n\n" + code

	req := openai.ChatCompletionRequest{
		Model:            openai.GPT4,
		Messages:         []openai.ChatCompletionMessage{{Role: "system", Content: "You are a helpful assistant that describes code. Do not use // or any other identifier."}, {Role: "user", Content: prompt}},
		MaxTokens:        256,
		Temperature:      0.5,
		N:                0,
		Stream:           false,
		Stop:             []string{},
		PresencePenalty:  0,
		FrequencyPenalty: 0,
		LogitBias:        map[string]int{},
		User:             "",
	}
	resp, err := c.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	// Surround the generated comment with /* and */
	//comment := "/* " + resp.Choices[0].Message.Content + " */"
	comment := resp.Choices[0].Message.Content

	return comment, nil // return the generated text
}
