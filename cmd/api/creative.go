package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"
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
	file, err := app.uploadFile(r)
	if err != nil {
		app.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	app.writeJSON(w, http.StatusOK, envelope{"uploaded": file}, nil)
}
