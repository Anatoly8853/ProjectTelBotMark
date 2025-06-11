package routers

import (
	"ProjectTelBotMark/internal/structs"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func (h *Handler) handlerProducts(c *gin.Context) {
	category := c.Query("category")
	sort := c.Query("sort")
	data, err := h.repo.HandlerProducts(category, sort)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//h.app.Log.Info(data)
	c.HTML(http.StatusOK, "products.html", gin.H{
		//"Title":    "Заказы Салаты корейские",
		"Categories": data.Categories,
		"Products":   "current",
		"data":       data})
}

func (h *Handler) handlerProductsPage(c *gin.Context) {
	search := c.Query("search")
	category := c.Query("category")
	sort := c.Query("sort")
	pageStr := c.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageStr)

	pageParams := structs.PageProduct{
		Search:   search,
		Category: category,
		Sort:     sort,
		Page:     page,
		Limit:    9,
	}

	products, pagination, err := h.repo.HandlerProductsPage(pageParams)
	if err != nil {
		h.app.Log.Error("Ошибка базы данных: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка базы данных"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"products":    products,
		"totalPages":  pagination.TotalPages,
		"currentPage": pagination.Page,
	})
}

func (h *Handler) showAddForm(c *gin.Context) {
	c.HTML(http.StatusOK, "form.html", nil)
}

func (h *Handler) addProduct(c *gin.Context) {
	title := c.PostForm("title")
	description := c.PostForm("description")
	price := c.PostForm("price")
	category := c.PostForm("category")
	packaging := c.PostForm("packaging")

	text := fmt.Sprintf("⚖%s\n💵%s рублей", packaging, price)
	callback := fmt.Sprintf("add_%s %s_1_%s", title, packaging, price)
	priceAt, _ := strconv.Atoi(price)

	file, err := c.FormFile("image")
	imagePath := ""

	if err == nil {
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
		tempPath := filepath.Join(UploadsDir, "temp_"+filename)
		finalPath := filepath.Join(UploadsDir, filename)

		// Сохраняем во временный файл
		if err = c.SaveUploadedFile(file, tempPath); err != nil {
			h.app.Log.Error("Ошибка при сохранении файла: " + err.Error())
			c.String(http.StatusInternalServerError, "Ошибка сохранения файла")
			return
		}

		// Обработка изображения
		src, err := imaging.Open(tempPath)
		if err != nil {
			h.app.Log.Error("Ошибка при открытии изображения: " + err.Error())
			c.String(http.StatusInternalServerError, "Неверный формат изображения")
			return
		}

		dst := imaging.Fill(src, 400, 400, imaging.Center, imaging.Lanczos)

		if err = imaging.Save(dst, finalPath); err != nil {
			h.app.Log.Error("Ошибка при сохранении обработанного изображения: " + err.Error())
			c.String(http.StatusInternalServerError, "Ошибка обработки изображения")
			return
		}

		_ = os.Remove(tempPath)

		// Записываем URL-адрес, а не локальный путь
		imagePath = "uploads/" + filename
	}

	product := structs.Product{
		Title:       title,
		Description: description,
		Price:       priceAt,
		Category:    category,
		Image:       imagePath,
		Packaging:   packaging,
		Text:        text,
		Callback:    callback,
	}
	err = h.repo.AddProduct(product)
	if err != nil {
		h.app.Log.Error("Database error: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
	}

	c.Redirect(http.StatusSeeOther, "/product")
}

func (h *Handler) editProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.repo.EditProduct(id)
	if err != nil {
		h.app.Log.Error("Database error: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
	}
	c.HTML(http.StatusOK, "form.html", gin.H{
		"p":           p,
		"Productedit": "current"})
}

func (h *Handler) updateProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	title := c.PostForm("title")
	description := c.PostForm("description")
	price := c.PostForm("price")
	category := c.PostForm("category")
	packaging := c.PostForm("packaging")

	text := fmt.Sprintf("⚖%s\n💵%s рублей", packaging, price)
	callback := fmt.Sprintf("add_%s %s_1_%s", title, packaging, price)
	priceAt, _ := strconv.Atoi(price)

	existingImage := c.PostForm("existing_image")
	file, err := c.FormFile("image")
	h.app.Log.Info("Путь к файлу изображения: ", existingImage)
	imagePath := ""
	if err == nil {

		// загружаем новый файл
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
		tempPath := filepath.Join(UploadsDir, "temp_"+filename)
		finalPath := filepath.Join(UploadsDir, filename)

		// Сохраняем во временный файл
		if err = c.SaveUploadedFile(file, tempPath); err != nil {
			h.app.Log.Error("Ошибка при сохранении файла: " + err.Error())
			c.String(http.StatusInternalServerError, "Ошибка сохранения файла")
			return
		}

		// Обработка изображения
		src, err := imaging.Open(tempPath)
		if err != nil {
			h.app.Log.Error("Ошибка при открытии изображения: " + err.Error())
			c.String(http.StatusInternalServerError, "Неверный формат изображения")
			return
		}

		dst := imaging.Fill(src, 400, 400, imaging.Center, imaging.Lanczos)

		if err = imaging.Save(dst, finalPath); err != nil {
			h.app.Log.Error("Ошибка при сохранении обработанного изображения: " + err.Error())
			c.String(http.StatusInternalServerError, "Ошибка обработки изображения")
			return
		}

		_ = os.Remove(tempPath)

		// Записываем URL-адрес, а не локальный путь
		imagePath = "uploads/" + filename
	} else {
		// используем старый путь
		imagePath = existingImage
	}

	p := structs.Product{
		ID:          id,
		Title:       title,
		Description: description,
		Price:       priceAt,
		Category:    category,
		Image:       imagePath,
		Packaging:   packaging,
		Text:        text,
		Callback:    callback,
	}

	err = h.repo.UpdateProduct(p)
	if err != nil {
		h.app.Log.Error("Ошибка обновления: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления"})
	}
	c.Redirect(http.StatusSeeOther, "/product")
}

func (h *Handler) deleteProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = h.repo.DeleteProduct(id)
	if err != nil {
		h.app.Log.Error("Ошибка удаления: " + err.Error())
	}
	c.Redirect(http.StatusSeeOther, "/product")
}

func (h *Handler) exportExcel(c *gin.Context) {
	f := excelize.NewFile()
	defer func(f *excelize.File) {
		if err := f.Close(); err != nil {
			h.app.Log.Error(err)
		}
	}(f)

	// --- Стили
	styles := map[string]*excelize.Style{
		"header": {
			Font:      &excelize.Font{Bold: true},
			Alignment: &excelize.Alignment{Horizontal: "center"},
			Border: []excelize.Border{
				{Type: "left", Style: 1, Color: "000000"},
				{Type: "right", Style: 1, Color: "000000"},
				{Type: "top", Style: 1, Color: "000000"},
				{Type: "bottom", Style: 1, Color: "000000"},
			},
		},
		"default": {
			Alignment: &excelize.Alignment{Horizontal: "center"},
			Border: []excelize.Border{
				{Type: "left", Style: 1, Color: "000000"},
				{Type: "right", Style: 1, Color: "000000"},
				{Type: "top", Style: 1, Color: "000000"},
				{Type: "bottom", Style: 1, Color: "000000"},
			},
		},
		"wrapText": {
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
				WrapText:   true,
			},
			Border: []excelize.Border{
				{Type: "left", Style: 1, Color: "000000"},
				{Type: "right", Style: 1, Color: "000000"},
				{Type: "top", Style: 1, Color: "000000"},
				{Type: "bottom", Style: 1, Color: "000000"},
			},
		},
	}

	styleIDs := make(map[string]int)
	for name, style := range styles {
		id, err := f.NewStyle(style)
		if err != nil {
			h.app.Log.Error("Ошибка создания стиля: " + name)
			c.String(http.StatusInternalServerError, "Ошибка стиля: "+name)
			return
		}
		styleIDs[name] = id
	}

	// --- Заголовки
	headers := []struct {
		Name  string
		Width float64
	}{
		{"№", 8},
		{"Название", 20},
		{"Цена", 10},
		{"Описание", 40},
		{"Категория", 20},
		{"Фасовка", 15},
	}

	for col, x := range headers {
		colName, _ := excelize.ColumnNumberToName(col + 1)
		cell := colName + "1"
		f.SetCellValue("Sheet1", cell, x.Name)
		f.SetCellStyle("Sheet1", cell, cell, styleIDs["header"])
		f.SetColWidth("Sheet1", colName, colName, x.Width)
	}

	// --- Данные
	rawData, err := h.repo.ExportExcel()
	if err != nil {
		h.app.Log.Error("Ошибка получения данных: " + err.Error())
		c.String(http.StatusInternalServerError, "Ошибка получения данных")
		return
	}

	// rawData — []interface{}, где каждый элемент это []interface{}
	rowNum := 2
	for _, raw := range rawData {
		row, ok := raw.([]interface{})
		if !ok || len(row) < 6 {
			continue
		}
		for col, val := range row {
			colName, _ := excelize.ColumnNumberToName(col + 1)
			cell := colName + strconv.Itoa(rowNum)

			if err := f.SetCellValue("Sheet1", cell, val); err != nil {
				h.app.Log.Error("Ошибка SetCellValue: " + err.Error())
				c.String(http.StatusInternalServerError, "Ошибка записи Excel")
				return
			}

			style := styleIDs["default"]
			if col == 3 { // описание
				style = styleIDs["wrapText"]
			}
			if err := f.SetCellStyle("Sheet1", cell, cell, style); err != nil {
				h.app.Log.Error("Ошибка SetCellStyle: " + err.Error())
				c.String(http.StatusInternalServerError, "Ошибка стилизации Excel")
				return
			}
		}
		rowNum++
	}

	// --- Отправка
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", `attachment; filename="products.xlsx"`)

	if err = f.Write(c.Writer); err != nil {
		h.app.Log.Error("Ошибка при выводе Excel: " + err.Error())
		c.String(http.StatusInternalServerError, "Ошибка при выводе Excel")
	}
}
