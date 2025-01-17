package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vishaaxl/cheershare/internal/data"
)

const MaxFileSize = 10 << 20

/**
 * generateUUIDFilename generates a unique filename based on a UUID.
 * It uses the original file's extension to keep the file format intact.
 * This helps avoid naming conflicts when storing uploaded files.
 */
func generateUUIDFilename(filePath string) string {
	ext := path.Ext(filePath)
	return fmt.Sprintf("%s%s", uuid.New().String(), ext)
}

/**
 * isImageFile checks whether the uploaded file is an image.
 * It checks the file extension to ensure that only .jpg, .jpeg, .png, and .gif files are allowed.
 */
func isImageFile(filename string) bool {
	allowedExtensions := []string{".jpg", ".jpeg", ".png", ".gif"}
	ext := strings.ToLower(path.Ext(filename))

	for _, validExt := range allowedExtensions {
		if ext == validExt {
			return true
		}
	}

	return false
}

/**
 * uploadFile handles the file upload logic.
 * It parses the incoming form data, checks for errors, and saves the file to disk.
 * The method also ensures that only images are uploaded by checking the file type.
 */
func (app *application) uploadFile(r *http.Request) (string, error) {
	err := r.ParseMultipartForm(MaxFileSize)
	if err != nil {
		/**
		 * If parsing the form fails (e.g., file size exceeds limit or form is malformed),
		 * return an empty string and the error to notify the caller.
		 */
		return "", err
	}

	/**
	 * Retrieve the uploaded file from the form data using the key "file".
	 * This returns the file object, its header (containing metadata like filename), and an error if any.
	 */
	file, header, err := r.FormFile("file")
	if err != nil {
		/**
		 * If there was an error retrieving the file (e.g., missing file in the request),
		 * return an empty string and the error to notify the caller.
		 */
		return "", err
	}
	defer file.Close()

	/**
	 * Check if the uploaded file is a valid image (using the file extension).
	 * The isImageFile function verifies that the file is one of the allowed image types (e.g., .jpg, .png).
	 */
	if !isImageFile(header.Filename) {
		return "", fmt.Errorf("invalid file type: only images are allowed")
	}

	/**
	 * Generate a unique filename for the uploaded file using a UUID to avoid naming conflicts.
	 * The generateUUIDFilename function ensures that each uploaded file gets a unique name,
	 * while retaining the file's original extension.
	 */
	uploadDir := "./uploads"
	destinationFilePath := fmt.Sprintf("%s/%s", uploadDir, generateUUIDFilename(header.Filename))

	/**
	 * Create the destination file in the specified upload directory.
	 * The os.Create function returns a file pointer for the newly created file.
	 */
	destinationFile, err := os.Create(destinationFilePath)
	if err != nil {
		return "", err
	}
	defer destinationFile.Close()

	/**
	 * Copy the contents of the uploaded file to the newly created destination file.
	 * The io.Copy function writes from the source (file) to the destination (destinationFile).
	 */
	_, err = io.Copy(destinationFile, file)
	if err != nil {
		return "", err
	}

	return destinationFilePath, nil
}

/**
 * uploadCreativeHandler handles the HTTP request for uploading a creative file.
 * It processes the file upload, checks for errors, and returns a response with the uploaded file path.
 */
func (app *application) uploadCreativeHandler(w http.ResponseWriter, r *http.Request) {
	/**
	 * Extract the "scheduled_at" parameter from the request form data.
	 * This represents the date when the creative will be scheduled.
	 * If the "scheduled_at" parameter is missing, respond with a 400 Bad Request error.
	 */
	scheduledAtStr := r.FormValue("scheduled_at")
	if scheduledAtStr == "" {
		app.errorResponse(w, http.StatusBadRequest, "scheduled_at is required")
		return
	}

	scheduledAt, err := time.Parse("2006-01-02", scheduledAtStr)
	if err != nil {
		app.errorResponse(w, http.StatusBadRequest, "invalid date format for scheduled_at")
		return
	}

	/**
	 * Validate that the scheduled date is not in the past.
	 * If the date is earlier than the current date, respond with a 400 Bad Request error.
	 */
	if scheduledAt.Before(time.Now()) {
		app.errorResponse(w, http.StatusBadRequest, "cannot set scheduled_at before today")
		return
	}

	/**
	 * Handle the file upload using the app.uploadFile method.
	 * If the file upload fails, respond with a 400 Bad Request error containing the error message.
	 */
	uploadedFile, err := app.uploadFile(r)
	if err != nil {
		app.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	creative := &data.Creative{
		CreativeURL: uploadedFile,
		ScheduledAt: scheduledAt,
		UserID:      app.contextGetUser(r).ID,
	}

	err = app.models.Creative.Insert(creative)
	if err != nil {
		app.logger.Println("Error saving creative:", err)
		app.errorResponse(w, http.StatusInternalServerError, "failed to save creative")
		return
	}

	app.writeJSON(w, http.StatusOK, envelope{"creative": creative}, nil)
}

func (app *application) getScheduledCreativesHandler(w http.ResponseWriter, r *http.Request) {
	scheduledCreatives, err := app.models.Creative.GetScheduledCreatives()
	if err != nil {

		app.errorResponse(w, http.StatusInternalServerError, "failed to fetch scheduled creatives")
		return
	}

	app.writeJSON(w, http.StatusOK, envelope{"scheduled_creatives": scheduledCreatives}, nil)
}
