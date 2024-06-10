package routers

import (
	"ProjectTelBotMark/structs"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
)

var categories map[string]structs.Category

func LoadCategories() {
	data, err := ioutil.ReadFile("data.json")
	if err != nil {
		logger.Fatalf("Error reading JSON file: %v", err)
	}
	if err = json.Unmarshal(data, &categories); err != nil {
		logger.Fatalf("Error unmarshalling JSON: %v", err)
	}
}

func saveCategories(c *gin.Context) {
	var newCategories map[string]structs.Category
	if err := c.ShouldBindJSON(&newCategories); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	data, err := json.Marshal(newCategories)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := ioutil.WriteFile("buttons.json", data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	categories = newCategories
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
