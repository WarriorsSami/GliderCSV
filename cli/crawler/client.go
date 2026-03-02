package crawler

import (
	"cli/internal/syscrawler"
	"fmt"
	"io"
	"strings"
)

type Client struct {
	crawler *syscrawler.CrawlerHandle
}

type Header = []string
type Row = []string

func Open(filePath string) (*Client, error) {
	crawler, err := syscrawler.OpenCrawler(filePath, 100, ',', "")
	if err != nil {
		return nil, err
	}
	
	return &Client{crawler: crawler}, nil
}

func (c *Client) NextBatch() (Header, []Row, error) {
	rec, err := c.crawler.NextBatch()
	if err != nil {
		if err == io.EOF {
			return nil, nil, io.EOF
		}
		return nil, nil, fmt.Errorf("failed to get next batch: %v", err)
	}
	defer rec.Release()

	schema := rec.Schema()
	header := make(Header, schema.NumFields())
	for i := 0; i < int(schema.NumFields()); i++ {
		header[i] = strings.Clone(schema.Field(i).Name)
	}

	rows := make([]Row, rec.NumRows())
	for i := 0; i < int(rec.NumRows()); i++ {
		row := make(Row, len(header))
		for j := 0; j < int(rec.NumCols()); j++ {
			col := rec.Column(j)
			if col.IsNull(i) {
				row[j] = ""
				continue
			}
			// strings.Clone copies the bytes out of the Arrow C buffer into
			// Go-owned memory so they survive the defer rec.Release() below.
			row[j] = strings.Clone(col.ValueStr(i))
		}
		rows[i] = row
	}

	return header, rows, nil
}

func (c *Client) Close() {
	c.crawler.Free()
}