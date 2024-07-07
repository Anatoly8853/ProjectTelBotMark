package routers

import (
	"ProjectTelBotMark/structs"
	"encoding/json"
	"net/http"
	`os`

	"github.com/gin-gonic/gin"
)

func LoadCategories(app *structs.Application) {

	data, err := os.ReadFile("buttons.json")
	if err != nil {
		app.Log.Errorf("Error reading JSON file: %v", err)
	}
	if err = json.Unmarshal(data, &structs.Categories); err != nil {
		app.Log.Errorf("Error unmarshalling JSON: %v", err)
	}
}

func saveCategories(c *gin.Context) {
	var newCategories map[string]structs.Category
	if err := c.ShouldBindJSON(&newCategories); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	data, err := json.MarshalIndent(newCategories, "", " ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err = os.WriteFile("buttons.json", data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	structs.Categories = newCategories
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
