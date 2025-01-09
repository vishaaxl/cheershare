package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
	"github.com/vishaaxl/cheershare/internal/data"
)

/*
generateOTP generates a 4-digit OTP using a secure random number generator.
The function creates a random byte array and uses modulus operation to generate
a number between 0 and 9999. The result is formatted into a zero-padded string
to ensure it is always 4 digits long.
*/
func generateOTP() string {
	otp := make([]byte, 2)

	_, err := rand.Read(otp)
	if err != nil {
		log.Fatal("Error generating OTP:", err)
	}
	return fmt.Sprintf("%04d", int(otp[0])%10000)
}

/*
storeOTPInRedis stores user data, including the OTP, into Redis.
It uses the phone number as the key and stores the data as a hash.
A timeout context is created to prevent blocking indefinitely, and
the key is set to expire after 5 minutes to ensure security.
*/
func (app *application) storeOTPInRedis(ctx context.Context, phoneNumber, name, otp string) error {
	userData := map[string]string{
		"name": name,
		"otp":  otp,
	}

	err := app.cache.HSet(ctx, phoneNumber, userData).Err()
	if err != nil {
		return fmt.Errorf("failed to store user data in Redis: %w", err)
	}

	err = app.cache.Expire(ctx, phoneNumber, 5*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration for Redis key: %w", err)
	}

	return nil
}

/*
verifyOTPInRedis retrieves user data from Redis and validates the provided OTP.
It ensures that the OTP matches the value stored in Redis for the given phone number.
If the OTP is invalid or the key does not exist, an error is returned.
*/
func (app *application) verifyOTPInRedis(ctx context.Context, phoneNumber, otp string) (string, error) {
	userData, err := app.cache.HGetAll(ctx, phoneNumber).Result()
	if err != nil || len(userData) == 0 {
		return "", fmt.Errorf("invalid or expired OTP")
	}

	storedOTP := userData["otp"]
	if otp != storedOTP {
		return "", fmt.Errorf("invalid OTP")
	}

	return userData["name"], nil
}

/*
createUserIfNotExists checks if a user already exists in the database for the provided phone number.
If the user exists, their record is returned. Otherwise, a new user is created with the given name
and phone number. Any errors during database operations are propagated back to the caller.
*/
func (app *application) createUserIfNotExists(phoneNumber, name string) (*data.User, error) {
	user, err := app.models.User.GetByPhoneNumber(phoneNumber)
	if err == nil && user != nil {
		return user, nil
	}

	newUser := data.User{
		Name:        name,
		PhoneNumber: phoneNumber,
	}

	err = app.models.User.Insert(&newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &newUser, nil
}

/*
generateTokenForUser creates a new authentication token for the given user ID.
The token is valid for 48 hours and is associated with the "authentication" scope.
If token generation fails, an error is returned to the caller.
*/
func (app *application) generateTokenForUser(userID int64) (string, error) {
	token, err := app.models.Token.New(userID, 48*time.Hour, data.ScopeAuthentication)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token.Plaintext, nil
}

/*
handleUserSignupAndVerification manages both user signup and OTP verification.
It combines the following functionality:
1. If OTP is not provided, it generates a new OTP and sends it to the user.
2. If OTP is provided, it validates the OTP and registers or retrieves the user.
3. Once the user is verified, an authentication token is created and sent as a response.

The process includes:
- Parsing and validating input data.
- Interacting with Redis for OTP storage and retrieval.
- Ensuring user data is either created or retrieved from the database.
- Generating an authentication token for successful verification.
- Returning appropriate error responses for any issues encountered.
*/
func (app *application) handleUserSignupAndVerification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		PhoneNumber string `json:"phone_number"`
		OTP         string `json:"otp"`
	}

	/*
		Read JSON from the request body.
		If an error occurs while reading, respond with a bad request error and log the issue.
	*/
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.errorResponse(w, http.StatusBadRequest, "Invalid request payload")
		app.logger.Println("Error reading JSON:", err)
		return
	}

	/*
		Validate required fields:
		- Phone number is always required.
		- Name is required only for OTP generation.
	*/
	if input.PhoneNumber == "" {
		app.errorResponse(w, http.StatusBadRequest, "Phone number is required")
		return
	}

	if input.OTP == "" {
		/*
			OTP is not provided; generate a new OTP.
			1. Validate that the name is provided (required for OTP generation).
			2. Generate a new OTP.
			3. Store the OTP and name in Redis using the phone number as the key.
			4. Set an expiration time of 5 minutes for the Redis entry.
			5. Return a success response indicating the OTP was sent.
		*/
		if input.Name == "" {
			app.errorResponse(w, http.StatusBadRequest, "Name is required for OTP generation")
			return
		}

		otp := generateOTP()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = app.storeOTPInRedis(ctx, input.PhoneNumber, input.Name, otp)
		if err != nil {
			app.errorResponse(w, http.StatusInternalServerError, "Failed to store OTP")
			app.logger.Println("Error storing OTP in Redis:", err)
			return
		}

		app.background(func() {
			err := app.sendOTPViaTwilio(otp, input.PhoneNumber)
			if err != nil {
				app.logger.Println("Error sending OTP via Twilio:", err)
			}
		})

		app.writeJSON(w, http.StatusOK, envelope{"success": true, "message": "OTP sent successfully"}, nil)
		return
	}

	/*
		OTP is provided; verify the OTP and proceed with user registration.
		1. Retrieve the stored OTP and user data from Redis using the phone number as the key.
		2. Validate the provided OTP against the stored OTP.
		3. If valid:
			a. Create or fetch the user in the database.
			b. Generate an authentication token for the user.
			c. Return a success response with user details and the token.
		4. If invalid, respond with an error indicating the OTP is invalid or expired.
	*/
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userName, err := app.verifyOTPInRedis(ctx, input.PhoneNumber, input.OTP)
	if err != nil {
		app.errorResponse(w, http.StatusUnauthorized, "Invalid or expired OTP")
		app.logger.Println("OTP verification failed for", input.PhoneNumber, ":", err)
		return
	}

	user, err := app.createUserIfNotExists(input.PhoneNumber, userName)
	if err != nil {
		app.errorResponse(w, http.StatusInternalServerError, "Failed to register user")
		app.logger.Println("Error registering user:", err)
		return
	}

	token, err := app.generateTokenForUser(user.ID)
	if err != nil {
		app.errorResponse(w, http.StatusInternalServerError, "Failed to generate authentication token")
		app.logger.Println("Error generating token for user ID", user.ID, ":", err)
		return
	}

	app.writeJSON(w, http.StatusOK, envelope{
		"success": true,
		"data":    user,
		"message": "User registered successfully",
		"token":   token,
	}, nil)
}

// sendOTPViaTwilio sends an OTP to the specified phone number using Twilio's messaging API.
//
// Parameters:
// - otp: The one-time password to be sent in the message body.
// - phoneNumber: The recipient's phone number without the country code.
//
// The function uses Twilio's Go SDK to create and send a message with the provided OTP.
// The `from` phone number is pre-configured to be a Twilio-registered number.
//
// Returns:
// - An error if the OTP could not be sent successfully.
// - nil if the message was sent without issues.
func (app *application) sendOTPViaTwilio(otp, phoneNumber string) error {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: os.Getenv("TWILIO_SID"),
		Password: os.Getenv("TWILIO_API_KEY"),
	})

	// Set up the parameters for the message.
	params := &api.CreateMessageParams{}
	params.SetBody(fmt.Sprintf(
		"Thank you for choosing Cheershare! Your one-time password is %v.",
		otp,
	))
	params.SetFrom(os.Getenv("TWILIO_PHONE_NUMBER")) // Twilio-registered phone number.
	params.SetTo(fmt.Sprintf("+91%v", phoneNumber))  // Format recipient's number with country code.

	const maxRetries = 3 // Number of retries
	var lastErr error    // Stores the last error encountered

	// Attempt to send the message with retries.
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := client.Api.CreateMessage(params)
		if err == nil {
			fmt.Printf("OTP sent successfully. Twilio response SID: %v\n", resp.Sid)
			return nil
		}

		// Log the error for debugging.
		lastErr = fmt.Errorf("attempt %d: failed to send OTP via Twilio: %w", attempt, err)
		fmt.Println(lastErr)

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("all retries failed to send OTP via Twilio: %w", lastErr)
}
