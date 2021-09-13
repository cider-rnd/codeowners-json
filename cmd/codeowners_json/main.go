package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hmarr/codeowners"
	"github.com/karrick/godirwalk"
	flag "github.com/spf13/pflag"
)

type File struct {
	Name   string   `json:"files"`
	Owners []string `json:"owners"`
}

type JsonResponse struct {
	Users      map[string][]string `json:"users"`
	OwnedFiles []File              `json:"owned_files"`
	Unowned    []string            `json:"unowned_files"`
}

func NewJsonResponse() *JsonResponse {
	var j JsonResponse
	j.Users = make(map[string][]string)
	return &j
}

func main() {
	var (
		ownerFilters   []string
		codeownersPath string
		helpFlag       bool
		output         string
	)
	flag.StringSliceVarP(&ownerFilters, "owner", "o", nil, "filter results by owner")
	flag.StringVarP(&codeownersPath, "file", "f", "", "CODEOWNERS file path")
	flag.BoolVarP(&helpFlag, "help", "h", false, "show this help message")
	flag.StringVarP(&output, "output", "t", "", "show this help message")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: codeowners <path>...\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	res := NewJsonResponse()

	ruleset, err := loadCodeowners(codeownersPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, rule := range ruleset {
		for _, owner := range rule.Owners {
			res.Users[owner.String()] = append(res.Users[owner.String()], rule.RawPattern())
		}
	}

	paths := flag.Args()
	if len(paths) == 0 {
		paths = append(paths, ".")
	}

	// Make the @ optional for GitHub teams and usernames
	for i := range ownerFilters {
		ownerFilters[i] = strings.TrimLeft(ownerFilters[i], "@")
	}

	for _, startPath := range paths {
		// godirwalk only accepts directories, so we need to handle files separately
		if !isDir(startPath) {
			file := getFileOwners(ruleset, startPath, ownerFilters)
			if file != nil {
				if len(file.Owners) > 0 {
					res.OwnedFiles = append(res.OwnedFiles, *file)
				} else {
					res.Unowned = append(res.Unowned, file.Name)
				}
			}
			continue
		}

		godirwalk.Walk(startPath, &godirwalk.Options{
			Callback: func(path string, dirent *godirwalk.Dirent) error {
				if path == ".git" {
					return filepath.SkipDir
				}

				// Only show code owners for files, not directories
				if !dirent.IsDir() {
					file := getFileOwners(ruleset, path, ownerFilters)
					if file != nil {
						if len(file.Owners) > 0 {
							res.OwnedFiles = append(res.OwnedFiles, *file)
						} else {
							res.Unowned = append(res.Unowned, file.Name)
						}
					}
				}
				return nil
			},
			Unsorted: true,
		})

		//jsonResponse, _ := json.MarshalIndent(res,"", "   ")
		jsonResponse, _ := json.Marshal(res)
		fmt.Printf(string(jsonResponse))

	}
}

func getFileOwners(ruleset codeowners.Ruleset, path string, ownerFilters []string) *File {
	rule, err := ruleset.Match(path)
	if err != nil {
		return nil
	}
	// If we didn't get a match, the file is unowned
	if rule == nil {
		// Don't show unowned files if we're filtering by owner
		//if len(ownerFilters) == 0 {
		//	fmt.Printf("%-70s  (unowned)\n", path)
		//}
		return &File{
			Name:   path,
			Owners: []string{},
		}
	}

	// Figure out which of the owners we need to show according to the --owner filters
	owners := []string{}
	for _, o := range rule.Owners {
		// If there are no filters, show all owners
		filterMatch := len(ownerFilters) == 0
		for _, filter := range ownerFilters {
			if filter == o.Value {
				filterMatch = true
			}
		}
		if filterMatch {
			owners = append(owners, o.String())
		}
	}

	return &File{
		Name:   path,
		Owners: owners,
	}
}

func loadCodeowners(path string) (codeowners.Ruleset, error) {
	if path == "" {
		return codeowners.LoadFileFromStandardLocation()
	}
	return codeowners.LoadFile(path)
}

// isDir checks if there's a directory at the path specified.
func isDir(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}
