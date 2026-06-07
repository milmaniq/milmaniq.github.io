package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/milmaniq/milmaniq.github.io/rest-api/models"
)

func main() {
	server := gin.Default()

	server.GET("/events", getEvents)
	server.POST("/events", createEvent)

	server.Run(":8080")
}

func getEvents(context *gin.Context) {
	events := models.GetAllEvents()
	context.JSON(http.StatusOK, events)
}

func createEvent(context *gin.Context) {
	var event models.Event
	// shouldBindJSON is a method that binds the JSON body to the event struct
	// if ID field is not present, then it will set the defaul null value which is 0
	// if Name filed is not present, then it will return an error as we have added a struct tag - binding:"required"
	if err := context.ShouldBindJSON(&event); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	event.ID = 1
	event.UserID = 1

	event.Save()
	context.JSON(http.StatusCreated, gin.H{"message": "Event created", "event": event})
}
