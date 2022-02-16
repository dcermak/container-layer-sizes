package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	internal "github.com/dcermak/container-layer-sizes/pkg"

	logrus "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

var log = logrus.New()

func backend(s *internal.SQLiteBackend) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			if err := r.ParseForm(); err != nil {
				http.Error(
					w,
					fmt.Sprintf("Error parsing form values %s", err),
					http.StatusBadRequest,
				)
				return
			}
			name := r.FormValue("name")
			id := r.FormValue("id")

			if (name == "" && id == "") || (name != "" && id != "") {
				http.Error(
					w,
					"Either the parameter id or name must be present",
					http.StatusBadRequest,
				)
				return
			}

			var payload interface{}
			if id != "" {
				var res *internal.ImageHistory

				i, err := strconv.ParseInt(id, 10, 64)
				if err != nil {
					log.WithFields(
						logrus.Fields{"id": id},
					).Error("Received an invalid id")
					http.Error(
						w,
						"could not parse id as int64",
						http.StatusBadRequest,
					)
					return
				}
				res, err = s.ReadById(i)
				if err != nil {
					if errors.Is(err, internal.ErrNonExistent) {
						http.Error(
							w,
							fmt.Sprintf(
								"No image history with the id %s, is present in the database",
								id,
							),
							http.StatusNotFound,
						)
					} else {
						http.Error(
							w,
							fmt.Sprintf(
								"Could not retrieve image with the id %s, got %s",
								id,
								err,
							),
							http.StatusInternalServerError,
						)
					}
					return
				}

				if res == nil {
					http.Error(w, fmt.Sprintf("No image history found with the id %s", id), http.StatusNotFound)
					return
				}
				payload = res
			} else if name != "" {
				var res []internal.ImageHistory
				res, err := s.Read(name)
				if err != nil {
					http.Error(w, fmt.Sprintf("Could not retrieve image with the name %s, got %s", name, err), http.StatusBadRequest)
					return
				}
				if len(res) == 0 {
					http.Error(w, fmt.Sprintf("No image history found with the name %s", name), http.StatusNotFound)
					return
				}
				payload = res
			}

			b, err := json.Marshal(payload)
			if err != nil {
				http.Error(w, "Failed to marshal the image history to json", http.StatusInternalServerError)
			} else {
				fmt.Fprint(w, string(b))
			}

			return
		case "PUT":
			fallthrough
		case "POST":
			var hist internal.ImageHistory
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(
					w,
					fmt.Sprintf("Failed to read the request body: %s", err),
					http.StatusInternalServerError,
				)
			}

			err = json.Unmarshal(body, &hist)
			if err != nil {
				http.Error(
					w,
					fmt.Sprintf(
						"Could not unmarshal the 'image_history' parameter from json, got: %s",
						err,
					),
					http.StatusBadRequest,
				)
				return
			}

			var res *internal.ImageHistory

			if r.Method == "PUT" {
				res, err = s.Create(&hist)
			} else if r.Method == "POST" {
				res, err = s.Update(&hist)
			}
			if err != nil {
				http.Error(
					w, fmt.Sprintf(
						"Failed to create or update the image history, got: %s",
						err,
					),
					http.StatusInternalServerError,
				)
			}

			b, err := json.Marshal(res)
			if err != nil {
				http.Error(w, "Failed to marshal the image history to json", http.StatusInternalServerError)
			} else {
				fmt.Fprint(w, string(b))
			}
			return
		case "DELETE":
			name := r.FormValue("name")
			err := s.DeleteByName(name)
			if err != nil {
				log.WithFields(
					logrus.Fields{"error": err, "name": name},
				).Error("Could not delete the image history with the given name")
				http.Error(
					w,
					fmt.Sprintf(
						"Could not retrieve an image history with the name %s",
						err,
					),
					http.StatusBadRequest,
				)
			} else {
				fmt.Fprint(w, "")
			}
			return

		}
	}
}

func main() {
	var addr, dbPath, verbosity string

	verbosityMap := map[string]logrus.Level{
		logrus.FatalLevel.String(): logrus.Level(logrus.FatalLevel),
		logrus.ErrorLevel.String(): logrus.Level(logrus.ErrorLevel),
		logrus.WarnLevel.String():  logrus.Level(logrus.WarnLevel),
		logrus.InfoLevel.String():  logrus.Level(logrus.InfoLevel),
		logrus.DebugLevel.String(): logrus.Level(logrus.DebugLevel),
		logrus.TraceLevel.String(): logrus.Level(logrus.TraceLevel),
	}

	app := cli.App{
		Name:  "storage-backend",
		Usage: "Launches the storage backend API server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "addr",
				Aliases:     []string{"a"},
				Usage:       "The address to which to bind",
				Value:       ":4040",
				Destination: &addr,
			},
			&cli.StringFlag{
				Name:        "sqlite-dbpath",
				Aliases:     []string{"path", "p"},
				Usage:       "Path to the sqlite database, defaults to 'database.sqlite3 in the current directory",
				Value:       "./database.sqlite3",
				Destination: &dbPath,
			},
			&cli.StringFlag{
				Name:        "verbosity",
				Aliases:     []string{"v"},
				Usage:       "Set the verbosity",
				Value:       logrus.WarnLevel.String(),
				Destination: &verbosity,
			},
		},
		Action: func(c *cli.Context) error {
			log.SetFormatter(&logrus.JSONFormatter{})
			if v, ok := verbosityMap[verbosity]; !ok {
				return errors.New(
					fmt.Sprintf("Invalid verbosity: %s", verbosity),
				)
			} else {
				log.SetLevel(v)
			}

			s, err := internal.CreateSQLiteBackend(dbPath)
			if err != nil {
				return err
			}
			http.HandleFunc("/", backend(s))

			fmt.Printf("Ready. Listening on %s\n", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				return err
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}

}
