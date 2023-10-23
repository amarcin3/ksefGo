//go:build ignore

package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html/charset"
)

func parseSchema(schemaFile string, outFilePath string) error {
	var numChildren int
	replacer := strings.NewReplacer(".xsd", "", "schemas/", "")
	var packageName = strings.ToLower(replacer.Replace(schemaFile))

	xmlFile, err := os.Open(schemaFile)
	if err != nil {
		return fmt.Errorf("cannot open source file: %v", err)
	}
	defer xmlFile.Close()

	decoder := xml.NewDecoder(xmlFile)
	decoder.CharsetReader = charset.NewReaderLabel

	outFile, err := os.OpenFile(outFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("cannot open dest file for writing: %v", err)
	}
	outFile.Truncate(0)
	defer outFile.Close()

	outFile.WriteString("// this file is generated by go generate. Please do not modify it manually!\n")
	outFile.WriteString(fmt.Sprintf("package %s\n", packageName))
	outFile.WriteString("\n")
	outFile.WriteString(fmt.Sprintf("var %sChildrenOrder = map[string]map[string]int{\n", strings.Replace(filepath.Base(schemaFile), ".xsd", "", 1)))
	for section, childrenNodes := range specNodeOrder(decoder) {
		outFile.WriteString("\t" + fmt.Sprintf(`"%s": {`, section))
		numChildren = len(childrenNodes)
		for i, child := range childrenNodes {
			outFile.WriteString(fmt.Sprintf(`"%s": %d`, child, i))
			if i < numChildren-1 {
				outFile.WriteString(",")
			}
		}
		outFile.WriteString("},\n")
	}
	outFile.WriteString("}")

	return nil
}

type NodeOrderTree struct {
	complexTypes  map[string]bool
	sequenceOrder map[string][]string
	nodeType      map[string]string
}

func (not *NodeOrderTree) recurseResolveNodeOrder() {
	for path, _type := range not.nodeType {
		fmt.Printf("type of %s is %s\n", path, _type)
		not.recurseResolveNodeOrderIterator(path, _type)
	}
}

func (not *NodeOrderTree) recurseResolveNodeOrderIterator(rootPath string, _type string) {
	order, exists := not.sequenceOrder[_type]
	if !exists {
		return
	}

	for _, field := range order {
		fieldType, exists := not.nodeType[_type+"."+field]
		if !exists {
			continue
		}
		not.recurseResolveNodeOrderIterator(rootPath+"."+field, fieldType)
	}

	not.sequenceOrder[rootPath] = order

}

// func (not *NodeOrderTree) resolveRecurseReplacementIterator(path string, typeName string) {
// 	_, exists := not.complexTypes[typeName]
// 	if !exists {
// 		return
// 	}

// 	fields := not.sequenceOrder[typeName]
// 	for field := range fields {
// 		resolveRecurseReplacementIterator(path + "." + field, "")
// 	}

// 	not.sequenceOrder[path] =
// }

func specNodeOrder(decoder *xml.Decoder) map[string][]string {
	var localPath = []string{}
	var nodeName string
	var fullPath string
	var elementName string

	var nodeOrderTree *NodeOrderTree = &NodeOrderTree{
		complexTypes:  make(map[string]bool),
		sequenceOrder: make(map[string][]string),
		nodeType:      make(map[string]string),
	}

	for {
		token, _ := decoder.Token()
		if token == nil {
			break
		}

		switch element := token.(type) {
		case xml.StartElement:
			elementName = element.Name.Local

			nodeName = elementName
			if name := getAttrib(element.Attr, "name"); name != "" {
				nodeName = name
			}

			// fmt.Printf("full path = %s\n", fullPath)

			if elementName == "complexType" {
				if name := getAttrib(element.Attr, "name"); name != "" {
					nodeOrderTree.complexTypes[name] = true
				}
			}

			// if nodeOrderTree.sequenceOrder[fullPath] == nil {
			// 	// fmt.Printf("create list of strings for %s\n", fullPath)
			// 	nodeOrderTree.sequenceOrder[fullPath] = make([]string, 0)
			// }

			if elementName == "sequence" {
				// fmt.Printf("found sequence at %s\n", fullPath)
				if nodeOrderTree.sequenceOrder[fullPath] == nil {
					nodeOrderTree.sequenceOrder[fullPath] = make([]string, 0)
				}

			} else if elementName == "element" {
				nodeName = getAttrib(element.Attr, "name")

				// fmt.Printf("appending %s to %s\n", nodeName, fullPath)
				nodeOrderTree.sequenceOrder[fullPath] = append(nodeOrderTree.sequenceOrder[fullPath], nodeName)

				// let's check if this is a complex type by any chance
				_type := getAttrib(element.Attr, "type")
				if _type != "" {
					_type = strings.Replace(_type, "tns:", "", 1)
					nodeOrderTree.nodeType[fullPath+"."+nodeName] = _type
					// append to the replacement array in case the actual type is defined
					// later than the field itself.
					// fmt.Printf("fullPAth=%s; node=%s; type=%s\n", fullPath, nodeName, _type)
					// recurseReplacement[fullPath] = _type
					// recurseReplacement[fullPath+"."+nodeName] = _type
				}
				// localPath = append(localPath, nodeName)
			}

			localPath = append(localPath, nodeName)
			fullPath = getFullPath(localPath)

			// fmt.Printf("currentPath = %s\n", strings.Join(localPath, "."))
		case xml.EndElement:
			localPath = localPath[0 : len(localPath)-1]
			fullPath = getFullPath(localPath)
			// fmt.Printf("currentPath = %s\n", strings.Join(localPath, "."))
		default:
		}
	}

	// fmt.Printf("complex types: %v\n", complexTypes)
	// for name, _ := range complexTypes {
	// 	fmt.Printf("order for type: %s = %v\n", name, sequenceOrder[name])
	// }
	// fmt.Printf("node types:\n")
	// for name, _type := range nodeType {
	// 	fmt.Printf("%s = %v\n", name, _type)
	// }
	// // check replacement sequences, if any:
	// func foo(path string, _type string) {
	// 	fields, exists := sequenceOrder[_type]
	// 	if !exists {
	// 		return
	// 	}

	// 	for field := range fields {
	// 		foo(path + "." + field)
	// 	}
	// }
	nodeOrderTree.recurseResolveNodeOrder()
	// for path, _type := range recurseReplacement {
	// 	if _, exists := sequenceOrder[_type]; !exists {
	// 		continue
	// 	}
	// 	fmt.Printf("recursing trough %s (%s)\n", path, _type)
	// 	// foo(path, _type)
	// }
	// // fmt.Printf("replace sequences: %+v\n", replaceSequences["AdresPol"])
	// for path, _type := range replaceSequences {
	// 	if sequenceOrder[_type] != nil {
	// 		sequenceOrder[path] = sequenceOrder[_type]
	// 	}
	// }

	return nodeOrderTree.sequenceOrder
}

func getAttrib(attribs []xml.Attr, name string) string {
	for _, attr := range attribs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}

	return ""
}

func getFullPath(path []string) string {
	var ignoredNodeNames = map[string]bool{
		"schema":        true,
		"complexType":   true,
		"sequence":      true,
		"element":       true,
		"enumeration":   true,
		"annotation":    true,
		"choice":        true,
		"restriction":   true,
		"documentation": true,
	}

	var fullPath = strings.Join(path, ".")

	for _, name := range path {
		if _, exists := ignoredNodeNames[name]; exists {
			fullPath = strings.ReplaceAll(fullPath, name+".", "")
			fullPath = strings.ReplaceAll(fullPath, "."+name, "")
		}
	}

	return fullPath
}

func main() {
	var err error

	files, err := os.ReadDir("schemas")
	if err != nil {
		log.Fatal(err)
	}

	var fileNameBase string

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".xsd") {
			fileNameBase = strings.Replace(file.Name(), ".xsd", "", 1)
			if err = parseSchema("schemas/"+file.Name(), "generators/"+fileNameBase+"/schema_ordering.go"); err != nil {
				log.Fatal(err)
			}
		}
	}
}
