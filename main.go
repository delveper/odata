package main

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type Book struct {
	ID        string `json:"id" sql:"id"`
	AuthorID  string `json:"author_id" sql:"author_id"`
	Title     string `json:"title" sql:"title"`
	Genre     string `json:"genre" sql:"genre"`
	Rate      int    `json:"rate" sql:"rate"`
	Size      int    `json:"size" sql:"size"`
	CreatedAt time.Time
}

func main() {
	var src Book
	q, err := getStructFieldNamesAndTags(src)
	if err != nil {
		log.Fatal(err)
	}
	// q := parseQueryOprion("$filter=Rate lt 100 and Rate ge 400 and Genre eq 'Thriller'&$orderby=Name desc&$top=100&$skip=10", filter)
	fmt.Println(q)
}

// DataFilter build on top of OData filter query options:
// $filter. represents filter which supports operations: `and`, `or`, `eq`, `ne`, `gt`, `lt`.
// Not yet supports following properties: `from`, `to` (in UTC format), `in` Sequences (ids of sequences).
// $orderby. optional param, represents sorting column which supports `acs` and `desc` operators.
// $top. optional param, represents limit of items from the resource.
// $skip. optional param, represents offset of records in the resource.
// Example: /books?$filter=Rate lt 100 and Rate ge 400 and Genre eq 'Thriller'&$orderby=Name desc&$top=100&$skip=10
type DataFilter struct {
	Filter  *Filter
	OrderBy *string
	Top     *int
	Skip    *int
}

type Filter struct {
	Head *Node
}

type Node struct {
	Field       string
	Operator    Operator
	Conjunction Conjunction
	Value       any
	Next        *Node
}

type QueryOption string
type Operator string
type Conjunction string

const (
	filter  QueryOption = "$filter"
	orderBy             = "$orderby"
	top                 = "$top"
	skip                = "$skip"
)
const (
	in Operator = "IN"
	eq          = "="
	ne          = "!="
	gt          = ">"
	lt          = "<"
)

const (
	and Conjunction = "AND"
	or              = "OR"
)
const defaultTagName = "sql"

func ParseFilter() {
	fields := strings.Join(nil, "|")

	pattern := fmt.Sprintf(`(?P<field>%s)\s+(?P<operator>%s)\s+(?P<value>\d+|'[^']+\')\s*(?P<conjunction>%s)*\s*`,
		fields,
		`in|eq|ne|gt|lt`,
		`and|or`,
	)

	_ = pattern
}

// getStructFieldNamesAndTags retrieves list of struct field names
// and their tag according to given tag name.
func getStructFieldNamesAndTags(src any) ([]string, error) {
	var res []string

	srcValue := reflect.Indirect(reflect.ValueOf(src))
	if srcType := srcValue.Kind(); srcType != reflect.Struct {
		return nil, fmt.Errorf("input value must be struct, got: %v", srcType)
	}

	// iterate struct fields.
	for i := 0; i < srcValue.NumField(); i++ {
		fieldValue := srcValue.Field(i)
		fieldName := srcValue.Type().Field(i).Name

		tag := srcValue.Type().Field(i).Tag
		tagValue := tag.Get(defaultTagName)

		// add only exported fields.
		if !unicode.IsUpper([]rune(fieldName)[0]) {
			continue
		}

		// add FieldName and value of defaultTagName.
		res = append(res, fieldName, tagValue)

		// recursive call for nested structs.
		if fieldValue.Type().Kind() != reflect.Struct {
			continue
		}

		nested, err := getStructFieldNamesAndTags(fieldValue.Interface())
		if err != nil {
			return nil, fmt.Errorf("error validating nested struct: %w", err)
		}

		res = append(res, nested...)
	}

	return res, nil
}

// parseQueryOption parses value of given QueryOption from URL query parameters.
func parseQueryOption(query string, opt QueryOption) string {
	/*
		query := url.Query()
		return query.Get(string(opt))
	*/
	pattern := fmt.Sprintf(`(?P<option>\%s=)(?P<value>[^&$]*)`, opt)
	if match := regexp.MustCompile(pattern).
		FindStringSubmatch(query); match != nil {
		return match[2]
	}

	return ""
}
