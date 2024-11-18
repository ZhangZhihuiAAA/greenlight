package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"greenlight.zzh.net/internal/validator"
)

func (app *application) readIDParam(r *http.Request) (int64, error) {
    params := httprouter.ParamsFromContext(r.Context())

    id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
    if err != nil || id < 1 {
        return 0, errors.New("invalid id parameter")
    }

    return id, nil
}

func (app *application) readString(qs url.Values, key, defaultValue string) string {
    // Extract the value for a given key from the query string. 
    // If no key exists this will return the empty string "".
    s := qs.Get(key)

    if s == "" {
        return defaultValue
    }

    return s
}

func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
    csv := qs.Get(key)

    if csv == "" {
        return defaultValue
    }

    return strings.Split(csv, ",")
}

func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
    s := qs.Get(key)

    if s == "" {
        return defaultValue
    }

    i, err := strconv.Atoi(s)
    if err != nil {
        v.AddError(key, "must be an integer value")
        return defaultValue
    }

    return i
}

type envelope map[string]any

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
    js, err := json.MarshalIndent(data, "", "    ")
    if err != nil {
        return err
    }

    // Append a newline to make it easier to view in terminal applications.
    js = append(js, '\n')

    // At this point, we know that we won't encounter any more errors before writing the response, 
    // so it's safe to add any headers that we want to include. We loop through the header map and 
    // add each header to the http.ResponseWriter header map. Note that it's OK if the provided 
    // header map is nil. Go doesn't throw an error if you try to range over a nil map.
    for key, value := range headers {
        w.Header()[key] = value  // w.Header().Set(key, value)
    }
    // iterator version
    // maps.Insert(w.Header(), maps.All(headers))

    // Add the "Content-Type: application/json" header, then write the status code and JSON 
    // response.
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    w.Write(js)

    return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
    // Use http.MaxBytesReader() to limit the size of the request body to 1MB.
    maxBytes := 1048576
    r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields()

    err := decoder.Decode(dst)
    if err != nil {
        // If there is an error during decoding, start the triage...
        var syntaxError *json.SyntaxError
        var unmarshalTypeError *json.UnmarshalTypeError
        var invalidUnmarshalError *json.InvalidUnmarshalError
        var maxBytesError *http.MaxBytesError

        switch {
        case errors.As(err, &syntaxError):
            return fmt.Errorf("body contains invalid JSON (at character %d)", syntaxError.Offset)

        case errors.Is(err, io.ErrUnexpectedEOF):
            return errors.New("body contains invalid JSON")

        case errors.As(err, &unmarshalTypeError):
            if unmarshalTypeError.Field != "" {
                return fmt.Errorf("body contains incorrect JSON type for field %s", unmarshalTypeError.Field)
            }
            return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

        case errors.Is(err, io.EOF):
            return errors.New("body must not be empty")

        // If the JSON contains a field which cannot be mapped to the target destination then 
        // Decode() will now return an error message in the format "json: unknown field "<name>"". 
        // We check for this, extract the field name from the error, and interpolate it into our 
        // custom error message. Not that there's an open issue at 
        // https://github.com/golang/go/issues/29035 regarding turning this into a distinct error 
        // type in the future.
        case strings.HasPrefix(err.Error(), "json: unknown field "):
            fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
            return fmt.Errorf("body contains unknown key %s", strings.Trim(fieldName, "\""))

        case errors.As(err, &maxBytesError):
            return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)

        case errors.As(err, &invalidUnmarshalError):
            panic(err)

        default:
            return err
        }
    }

    // Call Decode() again, using a pointer to an empty anonymous struct as the destination. If 
    // the request body only contained a single JSON value this will return an io.EOF error. So 
    // if we get anything else, we know that there is additional data in the rquest body and we 
    // return our own custom error message.
    err = decoder.Decode(&struct{}{})
    if !errors.Is(err, io.EOF) {
        return errors.New("body must only contain a single JSON value")
    }

    return nil
}