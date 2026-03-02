package ui

type BatchLoadedMsg struct {
	Header []string
	Rows   [][]string
}

type CrawlerErrorMsg struct {
	Err error
}

type EOFMsg struct{}