package ui

const (
	colorGreen  = "\u001B[32m"
	colorYellow = "\u001B[33m"
	colorRed    = "\u001B[31m"
	colorReset  = "\u001B[0m"
)

var Indent = indent{
	Small:  " ",
	Medium: "  ",
}

type indent struct {
	Small  string
	Medium string
}

type icons struct {
	Ok    string
	Info  string
	Error string
}
