package odata

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// TODO: Clear is better than clever.

// DataFilter represents a set of [OData](https://www.odata.org/getting-started/basic-tutorial/#queryData)
// query options to filter and sort data.
// It supports the following query options:
//   - $filter: optional parameter that represents a filter operation with operations: 'and', 'or', 'eq', 'ne', 'gt', 'lt', 'ge', 'le'.
//   - $orderby: optional parameter that represents a sorting column with operators: 'asc' and 'desc'.
//   - $top: optional parameter that represents a limit of items from the resource.
//   - $skip: optional parameter that represents an offset of records in the resource.
//
// The names of fields must correspond to struct field names and should be provided in case-sensitive format.
//
// Example: http://localhost:8080/books?$filter=Author eq 'Papa Karlo' and Title eq 'Pinocchio'&$orderby=Title desc&$skip=1&$top=10
/* TODO: add following properties: `from`, `to` (in UTC format), `in` Sequences (ids of sequences). */
type DataFilter struct {
	Filter  *Filter
	OrderBy string
	Top     int
	Skip    int
	URL     *url.URL
}

// Filter represent linked lists of OData expressions.
type Filter struct {
	RawQuery string
	Head     *FilterNode
}

// FilterNode represents OData expression.
type FilterNode struct {
	Field       string
	Operator    string
	Conjunction string
	Value       string
	Next        *FilterNode
}

type fieldData map[string]string

const (
	OptionFilter  = "$filter"
	OptionOrderBy = "$orderby"
	OptionTop     = "$top"
	OptionSkip    = "$skip"
)

const defaultTagName = "sql"

// NewDataFilter creates a new instance of *DataFilter of struct type T
// based on the OData query options present in the specified URL.
// The input of the OData query options will be validated during the process.
func NewDataFilter[T any](u *url.URL) (*DataFilter, error) {
	data, err := getStructFieldData(*new(T))
	if err != nil {
		return nil, err
	}

	filter, err := parseFilter(u, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", OptionFilter, err)
	}

	orderBy, err := parseOrderBy(u, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", OptionOrderBy, err)
	}

	top, err := parseTop(u)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", OptionTop, err)
	}

	skip, err := parseSkip(u)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", OptionSkip, err)
	}

	df := &DataFilter{
		URL:     u,
		Filter:  filter,
		OrderBy: orderBy,
		Top:     top,
		Skip:    skip,
	}

	return df, nil
}

// UpdateURL makes query on top of parent URL.
func (df *DataFilter) UpdateURL() {
	q := df.URL.Query()

	q.Set(OptionFilter, df.Filter.RawQuery)

	if df.Top != 0 {
		q.Set(OptionTop, fmt.Sprintf("%v", df.Top))
	}

	if df.Skip != 0 {
		q.Set(OptionSkip, fmt.Sprintf("%v", df.Skip))
	}

	df.URL.RawQuery = q.Encode()
}

// insert adds new expression to Filter chain.
func (f *Filter) insert(exp *FilterNode) {
	if f.Head == nil {
		f.Head = exp
		return
	}

	node := f.Head
	for node.Next != nil {
		node = node.Next
	}

	node.Next = exp
}

func parseSkip(u *url.URL) (int, error) {
	query := u.Query().Get(OptionSkip)
	if query == "" {
		return 0, nil
	}

	val, err := strconv.Atoi(query)
	if err != nil {
		return 0, fmt.Errorf("error parsing OptionSkip query option: %w", err)
	}

	return val, nil
}

func parseTop(u *url.URL) (int, error) {
	query := u.Query().Get(OptionTop)
	if query == "" {
		return 0, nil
	}

	val, err := strconv.Atoi(query)
	if err != nil {
		return 0, fmt.Errorf("error parsing OptionTop query option: %w", err)
	}

	return val, nil
}

func parseOrderBy(u *url.URL, fieldMap fieldData) (string, error) {
	query := u.Query().Get(OptionOrderBy)
	if query == "" {
		return "", nil
	}

	sortMap := map[string]string{
		"asc":  "ASC",
		"desc": "DESC",
		"ASC":  "ASC",
		"DESC": "DESC",
	}

	var fieldList, sortList []string
	for k, v := range fieldMap {
		fieldList = append(fieldList, k, v)
	}

	for k, v := range sortMap {
		sortList = append(sortList, v, k)
	}

	pattern := fmt.Sprintf(`(%s)(\s(%s))*,*`,
		strings.Join(fieldList, "|"),
		strings.Join(sortList, "|"),
	)

	re := regexp.MustCompile(pattern)

	if match := re.ReplaceAllLiteralString(query, ""); strings.TrimSpace(match) != "" {
		return "", fmt.Errorf("query does not correspond pattern: %s", pattern)
	}

	for k, v := range fieldMap {
		query = strings.ReplaceAll(query, k, v)
	}

	for k, v := range sortMap {
		query = strings.ReplaceAll(query, k, v)
	}

	return query, nil
}

func parseFilter(u *url.URL, fieldMap fieldData) (*Filter, error) {
	query := u.Query().Get(OptionFilter)
	if query == "" {
		return nil, nil
	}

	operMap := map[string]string{
		"eq": "=",
		"ne": "!=",
		"gt": ">",
		"lt": "<",
		"le": "<=",
		"ge": ">=",
	}

	conjMap := map[string]string{
		"and": "AND",
		"or":  "OR",
		"AND": "AND",
		"OR":  "OR",
	}

	fieldList := make([]string, 0, 2*len(fieldMap))
	for k, v := range fieldMap {
		fieldList = append(fieldList, k, v)
	}

	operList := make([]string, 0, len(operMap))
	for k := range operMap {
		operList = append(operList, k)
	}

	conjList := make([]string, 0, len(conjMap))
	for k := range conjMap {
		conjList = append(conjList, k)
	}

	pattern := fmt.Sprintf(`(?P<field>%s)\s+(?P<operator>%s)\s+(?P<value>\d+|'[^']+')\s*(?P<conjunction>%s)*\s*`,
		strings.Join(fieldList, "|"),
		strings.Join(operList, "|"),
		strings.Join(conjList, "|"),
	)

	re := regexp.MustCompile(pattern)
	if match := re.ReplaceAllLiteralString(query, ""); strings.TrimSpace(match) != "" {
		return nil, fmt.Errorf("query does not correspond pattern: %s", pattern)
	}

	matches := re.FindAllStringSubmatch(query, -1)
	groups := re.SubexpNames()

	f := &Filter{RawQuery: query}

	for _, match := range matches {
		var node FilterNode
		const skip = 1

		for i, group := range groups[skip:] {
			switch group {
			case "field":
				node.Field = fieldMap[match[i+skip]]
			case "operator":
				node.Operator = operMap[match[i+skip]]
			case "value":
				node.Value = match[i+skip]
			case "conjunction":
				node.Conjunction = conjMap[match[i+skip]]
			}
		}

		f.insert(&node)
	}

	return f, nil
}

// getStructFieldData retrieves a map of struct field names
// and tags of defaultTagName corresponding to them.
func getStructFieldData(src any) (fieldData, error) {
	res := make(map[string]string, 0)

	srcValue := reflect.Indirect(reflect.ValueOf(src))
	if srcType := srcValue.Kind(); srcType != reflect.Struct {
		return nil, fmt.Errorf("input value must be struct, got: %v", srcType)
	}

	// iterate struct fields.
	for i := 0; i < srcValue.NumField(); i++ {
		fieldValue := srcValue.Field(i)
		fieldName := srcValue.Type().Field(i).Name
		// add only exported fields.
		if r := string(fieldName[0]); r >= `a` && r <= `z` {
			continue
		}

		tag := srcValue.Type().Field(i).Tag
		tagValue := tag.Get(defaultTagName)
		// add only field with tags.
		if tagValue == "" {
			continue
		}

		// add FieldName and value of defaultTagName.
		res[fieldName] = tagValue

		// recursive call for nested structs.
		if fieldValue.Type().Kind() != reflect.Struct {
			continue
		}

		nested, err := getStructFieldData(fieldValue.Interface())
		if err != nil {
			return nil, fmt.Errorf("error validating nested struct: %w", err)
		}

		for k, v := range nested {
			res[k] = v
		}
	}

	return res, nil
}
