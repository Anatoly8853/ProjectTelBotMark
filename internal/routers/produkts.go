package routers

import (
	"ProjectTelBotMark/internal/structs"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// refreshCatalogCache перечитывает каталог из БД и безопасно публикует его для Telegram-бота.
func (h *Handler) refreshCatalogCache() error {
	categories, err := h.repo.GetCategoriesWithProducts()
	if err != nil {
		h.app.Log.Errorf("refreshCatalogCache: не удалось перечитать каталог из базы данных: %v", err)
		return err
	}
	structs.SetCategories(categories)
	h.app.Log.Infof("refreshCatalogCache: кэш каталога обновлён: категорий=%d", len(categories))
	return nil
}

func (h *Handler) handlerProducts(c *gin.Context) {
	category := c.Query("category")
	sort := c.Query("sort")
	data, err := h.repo.HandlerProducts(category, sort)
	if err != nil {
		h.app.Log.Errorf("handlerProducts: ошибка получения каталога: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения каталога"})
		return
	}

	c.HTML(http.StatusOK, "products.html", gin.H{
		//"Title": "Заказы Салаты корейские",
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
	options, err := h.repo.GetProductFormOptions()
	if err != nil {
		h.app.Log.Errorf("showAddForm: ошибка получения подсказок для формы товара: %v", err)
		c.String(http.StatusInternalServerError, "Не удалось загрузить данные формы товара")
		return
	}
	c.HTML(http.StatusOK, "form.html", gin.H{
		"p":       structs.Product{},
		"Options": options,
	})
}

// parseProductForm читает и проверяет общие поля формы добавления и редактирования товара.
func parseProductForm(c *gin.Context) (structs.Product, error) {
	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	category := strings.TrimSpace(c.PostForm("category"))
	packaging := strings.TrimSpace(c.PostForm("packaging"))
	priceRaw := strings.TrimSpace(c.PostForm("price"))
	price, err := strconv.Atoi(priceRaw)
	if title == "" || description == "" || category == "" || packaging == "" || err != nil || price <= 0 {
		return structs.Product{}, fmt.Errorf("заполните все обязательные поля и укажите положительную цену")
	}

	return structs.Product{
		Title: title, Description: description, Price: price, Category: category,
		Packaging: packaging,
		Text:      fmt.Sprintf("⚖%s\n💵%s рублей", packaging, priceRaw),
	}, nil
}

func (h *Handler) addProduct(c *gin.Context) {
	product, err := parseProductForm(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	imagePath, err := h.saveUploadedImage(c, "image")
	if err != nil {
		h.app.Log.Errorf("addProduct: ошибка загрузки изображения: %v", err)
		c.String(http.StatusBadRequest, "Не удалось загрузить изображение: %v", err)
		return
	}

	product.Image = imagePath
	if err := h.repo.AddProduct(product); err != nil {
		h.app.Log.Errorf("addProduct: ошибка сохранения товара: %v", err)
		if imagePath != "" {
			_ = os.Remove(filepath.Join(UploadsDir, filepath.Base(imagePath)))
		}
		c.String(http.StatusInternalServerError, "Не удалось сохранить товар")
		return
	}
	h.app.Log.Infof("addProduct: товар добавлен: название=%q, категория=%q", product.Title, product.Category)
	if err := h.refreshCatalogCache(); err != nil {
		c.String(http.StatusInternalServerError, "Изменения сохранены в базе данных, но каталог бота не обновлён. Не повторяйте операцию; проверьте журнал приложения.")
		return
	}
	c.Redirect(http.StatusSeeOther, "/product")
}

func (h *Handler) editProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID товара"})
		return
	}
	p, err := h.repo.EditProduct(id)
	if err != nil {
		h.app.Log.Errorf("editProduct: ошибка получения товара id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить товар"})
		return
	}
	options, err := h.repo.GetProductFormOptions()
	if err != nil {
		h.app.Log.Errorf("editProduct: ошибка получения подсказок для формы товара id=%d: %v", id, err)
		c.String(http.StatusInternalServerError, "Не удалось загрузить данные формы товара")
		return
	}
	c.HTML(http.StatusOK, "form.html", gin.H{
		"p":           p,
		"Options":     options,
		"Productedit": "current",
	})
}

func (h *Handler) updateProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.String(http.StatusBadRequest, "Некорректный ID товара")
		return
	}
	product, err := parseProductForm(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	product.ID = id

	imagePath, err := h.saveUploadedImage(c, "image")
	if err != nil {
		h.app.Log.Errorf("updateProduct: ошибка загрузки изображения: id=%d, ошибка=%v", id, err)
		c.String(http.StatusBadRequest, "Не удалось загрузить изображение: %v", err)
		return
	}
	newImageUploaded := imagePath != ""
	if imagePath == "" {
		imagePath = strings.TrimSpace(c.PostForm("existing_image"))
		if imagePath != "" && !isSafeProductImagePath(imagePath) {
			c.String(http.StatusBadRequest, "Некорректный путь к изображению")
			return
		}
	}

	product.Image = imagePath
	if err := h.repo.UpdateProduct(product); err != nil {
		h.app.Log.Errorf("updateProduct: ошибка обновления товара id=%d: %v", id, err)
		if newImageUploaded {
			_ = os.Remove(filepath.Join(UploadsDir, filepath.Base(imagePath)))
		}
		c.String(http.StatusInternalServerError, "Не удалось обновить товар")
		return
	}
	h.app.Log.Infof("updateProduct: товар обновлён: id=%d, название=%q", id, product.Title)
	if err := h.refreshCatalogCache(); err != nil {
		c.String(http.StatusInternalServerError, "Изменения сохранены в базе данных, но каталог бота не обновлён. Не повторяйте операцию; проверьте журнал приложения.")
		return
	}
	c.Redirect(http.StatusSeeOther, "/product")
}

// saveUploadedImage проверяет размер и формат файла, сохраняет обработанную копию с безопасным именем.
func (h *Handler) saveUploadedImage(c *gin.Context, fieldName string) (string, error) {
	file, err := c.FormFile(fieldName)
	if err == http.ErrMissingFile {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	if file.Size <= 0 || file.Size > 8<<20 {
		return "", fmt.Errorf("размер файла должен быть от 1 байта до 8 МБ")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return "", fmt.Errorf("поддерживаются только JPG и PNG")
	}
	opened, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("не удалось открыть загруженный файл: %w", err)
	}
	imageConfig, _, decodeErr := image.DecodeConfig(opened)
	closeErr := opened.Close()
	if decodeErr != nil {
		return "", fmt.Errorf("файл не является корректным изображением: %w", decodeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("не удалось закрыть загруженный файл: %w", closeErr)
	}
	if imageConfig.Width <= 0 || imageConfig.Height <= 0 || int64(imageConfig.Width)*int64(imageConfig.Height) > 40_000_000 {
		return "", fmt.Errorf("слишком большие размеры изображения")
	}
	if err := os.MkdirAll(UploadsDir, 0o755); err != nil {
		return "", fmt.Errorf("не удалось создать каталог загрузок: %w", err)
	}
	tmpPath := filepath.Join(UploadsDir, fmt.Sprintf("tmp_%d%s", time.Now().UnixNano(), ext))
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		return "", fmt.Errorf("не удалось сохранить временный файл: %w", err)
	}
	defer func() { _ = os.Remove(tmpPath) }()
	src, err := imaging.Open(tmpPath)
	if err != nil {
		return "", fmt.Errorf("файл не является корректным изображением: %w", err)
	}
	if src.Bounds().Dx() <= 0 || src.Bounds().Dy() <= 0 || int64(src.Bounds().Dx())*int64(src.Bounds().Dy()) > 40_000_000 {
		return "", fmt.Errorf("слишком большие размеры изображения")
	}
	dst := imaging.Fill(src, 400, 400, imaging.Center, imaging.Lanczos)
	filename := fmt.Sprintf("img_%d.jpg", time.Now().UnixNano())
	finalPath := filepath.Join(UploadsDir, filename)
	if err := imaging.Save(dst, finalPath); err != nil {
		_ = os.Remove(finalPath)
		return "", fmt.Errorf("не удалось обработать изображение: %w", err)
	}
	return "uploads/" + filename, nil
}

func isSafeProductImagePath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean != path || strings.Contains(path, "..") {
		return false
	}
	return strings.HasPrefix(path, "uploads/") || strings.HasPrefix(path, "jpeg/")
}

func (h *Handler) deleteProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID товара"})
		return
	}
	err = h.repo.DeleteProduct(id)
	if err != nil {
		h.app.Log.Errorf("deleteProduct: ошибка удаления товара id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить товар"})
		return
	}
	h.app.Log.Infof("deleteProduct: товар удалён: id=%d", id)
	if err := h.refreshCatalogCache(); err != nil {
		c.String(http.StatusInternalServerError, "Изменения сохранены в базе данных, но каталог бота не обновлён. Не повторяйте операцию; проверьте журнал приложения.")
		return
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
		colName, err := excelize.ColumnNumberToName(col + 1)
		if err != nil {
			h.app.Log.Errorf("exportExcel: не удалось определить имя столбца %d: %v", col+1, err)
			c.String(http.StatusInternalServerError, "Ошибка формирования Excel")
			return
		}
		cell := colName + "1"
		if err := f.SetCellValue("Sheet1", cell, x.Name); err != nil {
			h.app.Log.Errorf("exportExcel: ошибка записи заголовка %q: %v", x.Name, err)
			c.String(http.StatusInternalServerError, "Ошибка записи заголовка Excel")
			return
		}
		if err := f.SetCellStyle("Sheet1", cell, cell, styleIDs["header"]); err != nil {
			h.app.Log.Errorf("exportExcel: ошибка оформления заголовка %q: %v", x.Name, err)
			c.String(http.StatusInternalServerError, "Ошибка оформления заголовка Excel")
			return
		}
		if err := f.SetColWidth("Sheet1", colName, colName, x.Width); err != nil {
			h.app.Log.Errorf("exportExcel: ошибка установки ширины столбца %s: %v", colName, err)
			c.String(http.StatusInternalServerError, "Ошибка настройки столбцов Excel")
			return
		}
	}

	// --- Данные
	rawData, err := h.repo.ExportExcel()
	if err != nil {
		h.app.Log.Error("Ошибка получения данных: " + err.Error())
		c.String(http.StatusInternalServerError, "Ошибка получения данных")
		return
	}

	// rawData — срез строк Excel, каждая строка содержит значения any.
	rowNum := 2
	for _, raw := range rawData {
		row, ok := raw.([]any)
		if !ok || len(row) < 6 {
			continue
		}
		for col, val := range row {
			colName, err := excelize.ColumnNumberToName(col + 1)
			if err != nil {
				h.app.Log.Errorf("exportExcel: не удалось определить имя столбца %d: %v", col+1, err)
				c.String(http.StatusInternalServerError, "Ошибка формирования Excel")
				return
			}
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
