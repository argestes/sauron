package extractor

import (
	"bufio"
	"gopkg.in/yaml.v2"
	"io"
	"github.com/PuerkitoBio/goquery"
	"strings"
	"log"
	"fmt"
)

type Query struct {
	Selector        string  `yaml:"selector"`
	Name            string  `yaml:"name"`
	ForEachChildren bool    `yaml:"forEachChildren"`
	SubQueries      []Query `yaml:"subQueries"`
	Trim            bool    `yaml:"trim,omitempty"`
}

type HtmlExtractor struct {
	Name    string  `yaml:"name"`
	Queries []Query `yaml:"queries"`
}

func openDocument(reader *bufio.Reader) (Queryable, error) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	return &DocumentWrapper{doc}, nil
}

func (he *HtmlExtractor) Extract(reader *bufio.Reader) (*Field, error) {
	doc, err := openDocument(reader)
	if err != nil {
		return nil, err
	}

	rootQuery := Query{
		Selector:        "",
		Name:            he.Name,
		ForEachChildren: false,
		SubQueries:      he.Queries,
		Trim:            false,
	}

	f, err := executeQuery(doc, rootQuery)
	if err != nil {
		return nil, err
	}

	return f, nil
}

type SelectionWrapper struct {
	*goquery.Selection
}

type DocumentWrapper struct {
	*goquery.Document
}

type Queryable interface {
	F(string) Queryable
	Text() string
	EachQ(f func(int, Queryable)) Queryable
	ChildrenQ() Queryable
	DirectChildCount() int
}

func wrapSelection(selection *goquery.Selection) Queryable {
	return &SelectionWrapper{selection}
}

func wrapDocument(document *goquery.Document) Queryable {
	return &DocumentWrapper{document}
}

func (sw *SelectionWrapper) DirectChildCount() int {
	return len(sw.Nodes)
}

func (dw *DocumentWrapper) DirectChildCount() int {
	return len(dw.Nodes)
}

func (sw *SelectionWrapper) ChildrenQ() Queryable {
	return wrapSelection(sw.Children())
}

func (dw *DocumentWrapper) ChildrenQ() Queryable {
	return wrapSelection(dw.Children())
}

func (sw *SelectionWrapper) EachQ(f func(int, Queryable)) Queryable {
	q := sw.Each(func(i int, selection *goquery.Selection) {
		f(i, &SelectionWrapper{selection})
	})

	return &SelectionWrapper{q}
}

func (dw *DocumentWrapper) EachQ(f func(int, Queryable)) Queryable {
	q := dw.Each(func(i int, selection *goquery.Selection) {
		f(i, &SelectionWrapper{selection})
	})

	return &SelectionWrapper{q}
}

func (sw *SelectionWrapper) F(query string) Queryable {
	return &SelectionWrapper{sw.Find(query)}
}

func (dw *DocumentWrapper) F(query string) Queryable {
	return &SelectionWrapper{dw.Find(query)}
}

func executeSubqueries(document Queryable, queries []Query) ([]Field, error) {
	var fields []Field
	for _, v := range queries {
		f, err := executeQuery(document, v)
		if err != nil {
			return nil, err
		}

		if f != nil {
			fields = append(fields, *f)
		}
	}

	return fields, nil
}

func executeQuery(document Queryable, query Query) (*Field, error) {
	node := document.F(query.Selector)
	if query.Selector == "" {
		node = document
	}

	if query.ForEachChildren {
		var f Field
		f.Label = query.Name
		children := node.ChildrenQ()
		children.EachQ(func(i int, queryable Queryable) {
			subresults, err := executeSubqueries(queryable, query.SubQueries)
			if err != nil {
				log.Fatal(err)
			}

			fff := Field{
				Label:     fmt.Sprintf("%d", i),
				Data:      "",
				Subfields: subresults,
			}

			f.Subfields = append(f.Subfields, fff)
		})

		return &f, nil
	}

	if len(query.SubQueries) == 0 {
		var f Field
		f.Label = query.Name
		if node.DirectChildCount() == 0 {
			return nil, nil
		}
		dt := node.Text()
		if query.Trim {
			dt = strings.TrimSpace(dt)
		}

		f.Data = dt
		return &f, nil
	}

	subresults, err := executeSubqueries(node, query.SubQueries)
	if err != nil {
		return nil, err
	}

	var f Field
	f.Subfields = append(f.Subfields, subresults...)
	f.Label = query.Name
	return &f, nil
}

func NewHtmlExtractor(reader io.Reader) Extractor {
	var htmlExtractor HtmlExtractor
	yaml.NewDecoder(reader).Decode(&htmlExtractor)
	return &htmlExtractor
}