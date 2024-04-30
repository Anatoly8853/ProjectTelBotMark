package main

import "time"

type ButtonData struct {
	Text     string `json:"text"`
	Callback string `json:"callback"`
}

// 2. Распаковка данных
var Categories map[string]struct {
	CategoryName string       `json:"categoryName"`
	Photo        string       `json:"photo"`
	Buttons      []ButtonData `json:"buttons"`
	ButtonsM     []ButtonData `json:"buttonsM"`
}

type Purchase struct {
	Item      string
	Quantity  int
	Cost      int // Стоимость одного товара
	Timestamp time.Time
}
