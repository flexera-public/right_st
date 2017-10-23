package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/rightscale/rsc/cm15"
	"github.com/rightscale/rsc/rsapi"
)

type Alert struct {
	Name        string `yaml:"Name"`
	Description string `yaml:"Description,omitempty"`
	Clause      string `yaml:"Clause"`
	File        string `yaml:"-"`
}

type Alerts struct {
	Alerts []*Alert `yaml:"Alerts"`
}

func (alert *Alert) UnmarshalYAML(unmarshal func(interface{}) error) error {
	err := unmarshal(&alert.File)
	if err == nil {
		return nil
	}

	// make a dummy struct that will parse the same as Alert so we can unmarshall it without having UnmarshalYAML infinitely recursing
	var mapAlert struct {
		Name        string `yaml:"Name"`
		Description string `yaml:"Description,omitempty"`
		Clause      string `yaml:"Clause"`
	}
	err = unmarshal(&mapAlert)
	if err != nil {
		return err
	}
	alert.Name = mapAlert.Name
	alert.Description = mapAlert.Description
	alert.Clause = mapAlert.Clause
	alert.File = ""
	return nil
}

// ExpandAlerts goes through a slice of Alert structs and for any that have a File reference, it reads in
// Alert structs from the file and recurses through those to see if there are more File references and returns the new
// slice. It keeps track of which files it opens so it does not read the same file twice.
func ExpandAlerts(dir string, alerts []*Alert) ([]*Alert, error) {
	return expandAlerts(dir, make(map[string]bool), alerts)
}

func expandAlerts(dir string, files map[string]bool, alerts []*Alert) ([]*Alert, error) {
	expandedAlerts := make([]*Alert, 0, len(alerts))
	for _, alert := range alerts {
		if alert.File == "" {
			expandedAlerts = append(expandedAlerts, alert)
			continue
		}
		if files[alert.File] {
			continue
		} else {
			files[alert.File] = true
		}

		bytes, err := ioutil.ReadFile(filepath.Join(dir, alert.File))
		if err != nil {
			return nil, err
		}
		var container Alerts
		err = yaml.UnmarshalStrict(bytes, &container)
		if err != nil {
			return nil, err
		}
		if len(container.Alerts) == 0 {
			return nil, fmt.Errorf("alerts file does not contain any alerts: %s", alert.File)
		}
		alerts, err := expandAlerts(dir, files, container.Alerts)
		if err != nil {
			return nil, err
		}
		expandedAlerts = append(expandedAlerts, alerts...)
	}
	return expandedAlerts, nil
}

// Expected Format with array index offsets into tokens array below:
// If <Metric>.<ValueType> <ComparisonOperator> <Threshold> for <Duration> minutes Then <Escalate|Grow|Shrink> <ActionValue>
// 0  1                       2                    3           4   5          6       7    8                      9
func parseAlertClause(alert string) (*cm15.AlertSpec, error) {
	alertSpec := new(cm15.AlertSpec)
	tokens := strings.SplitN(alert, " ", 10)
	alertFmt := `If <Metric>.<ValueType> <ComparisonOperator> <Threshold> for <Duration> minutes Then <Action> <ActionValue>`
	if len(tokens) != 10 {
		return nil, fmt.Errorf("Alert clause misformatted: not long enough. Must be of format: '%s'", alertFmt)
	}
	if strings.ToLower(tokens[0]) != "if" {
		return nil, fmt.Errorf("Alert clause misformatted: missing If. Must be of format: '%s'", alertFmt)
	}
	metricTokens := strings.Split(tokens[1], ".")
	if len(metricTokens) != 2 {
		return nil, fmt.Errorf("Alert <Metric>.<ValueType> misformatted, should be like 'cpu-0/cpu-idle.value'.")
	}
	// Check metricTokens[0] should contain a slash.
	// Check metricTokens[1] can be numerous types: count, cumulative_requests, current_session, free
	//   midterm, percent, processes, read, running, rx, tx, shortterm, state, status, threads,
	//   used, users, value, write
	alertSpec.File = metricTokens[0]
	alertSpec.Variable = metricTokens[1]
	comparisonValues := []string{">", ">=", "<", "<=", "==", "!="}
	foundValue := false
	for _, val := range comparisonValues {
		if tokens[2] == val {
			foundValue = true
		}
	}
	if !foundValue {
		return nil, fmt.Errorf("Alert <ComparisonOperator> must be one of the following comparison operators: %s", strings.Join(comparisonValues, ", "))
	}
	alertSpec.Condition = tokens[2]
	// Threshold must be one of NaN, numeric OR booting, decommission, operational, pending, stranded, terminated
	alertSpec.Threshold = tokens[3]
	if strings.ToLower(tokens[4]) != "for" {
		return nil, fmt.Errorf("Alert clause misformatted, missing 'for'. Must be of format: '%s'", alertFmt)
	}
	duration, err := strconv.Atoi(tokens[5])
	if err != nil || duration < 1 {
		return nil, fmt.Errorf("Alert <Duration> must be a positive integer > 0")
	}
	alertSpec.Duration = duration

	if strings.Trim(strings.ToLower(tokens[6]), ",") != "minutes" {
		return nil, fmt.Errorf("Alert clause misformatted: missing 'minutes'. Must be of format: '%s'", alertFmt)
	}
	if strings.ToLower(tokens[7]) != "then" {
		return nil, fmt.Errorf("Alert clause misformatted: missing 'Then'. Must be of format: '%s'", alertFmt)
	}
	token8 := strings.ToLower(tokens[8])
	if token8 != "escalate" && token8 != "grow" && token8 != "shrink" {
		return nil, fmt.Errorf("Alert <Action> must be escalate, grow, or shrink")
	}
	if token8 == "escalate" {
		alertSpec.EscalationName = tokens[9]
	} else {
		alertSpec.VoteType = token8
		alertSpec.VoteTag = tokens[9]
	}
	return alertSpec, nil
}

// Complement to parseAlertClause
// If <Metric>.<ValueType> <ComparisonOperator> <Threshold> for <Duration> minutes Then <Escalate|Grow|Shrink> <ActionValue>
// 0  1                       2                    3           4   5          6       7    8                      9
func printAlertClause(as cm15.AlertSpec) string {
	var asAction, asActionValue string
	if as.EscalationName != "" {
		asAction = "escalate"
		asActionValue = as.EscalationName
	} else {
		asAction = as.VoteType
		asActionValue = as.VoteTag
	}
	alertStr := fmt.Sprintf("If %s.%s %s %s for %d minutes Then %s %s",
		as.File, as.Variable, as.Condition, as.Threshold, as.Duration, asAction, asActionValue)
	return alertStr
}

// Make sure an alert as described in yaml file on disk are correctly structured.
func validateAlert(alert *Alert) error {
	if alert.Name == "" {
		return fmt.Errorf("Name field must be present")
	}
	_, err := parseAlertClause(alert.Clause)
	if err != nil {
		return err
	}
	return nil
}

// Synchronizes alerts from the API to yaml file on disk
func downloadAlerts(st *cm15.ServerTemplate) ([]*Alert, error) {
	client, _ := Config.Account.Client15()

	alertsLocator := client.AlertSpecLocator(getLink(st.Links, "alert_specs"))
	alertSpecs, err := alertsLocator.Index(rsapi.APIParams{})
	if err != nil {
		return nil, fmt.Errorf("Could not find Alerts with href %s: %s", alertsLocator.Href, err.Error())
	}
	alerts := make([]*Alert, len(alertSpecs))
	for i, alertSpec := range alertSpecs {
		alerts[i] = &Alert{
			Name:        alertSpec.Name,
			Description: removeCarriageReturns(alertSpec.Description),
			Clause:      printAlertClause(*alertSpec),
		}
	}
	return alerts, nil
}

// Synchronizes alerts from yaml file on disk up to the API
func uploadAlerts(stDef *ServerTemplate) error {
	client, _ := Config.Account.Client15()

	alertsLocator := client.AlertSpecLocator(stDef.href + "/alert_specs")
	existingAlerts, err := alertsLocator.Index(rsapi.APIParams{})
	if err != nil {
		return fmt.Errorf("Could not find AlertSpecs with href %s: %s", alertsLocator.Href, err.Error())
	}
	seenAlert := make(map[string]bool)
	alertLookup := make(map[string]*cm15.AlertSpec)
	for _, alert := range existingAlerts {
		alertLookup[alert.Name] = alert
	}
	// Add/Update alerts
	for _, alert := range stDef.Alerts {
		parsedAlert, _ := parseAlertClause(alert.Clause)
		seenAlert[alert.Name] = true
		existingAlert, ok := alertLookup[alert.Name]
		if ok { // update
			if alert.Clause != printAlertClause(*existingAlert) || alert.Description != existingAlert.Description {
				alertsUpdateLocator := client.AlertSpecLocator(getLink(existingAlert.Links, "self"))

				fmt.Printf("  Updating Alert %s\n", alert.Name)
				params := cm15.AlertSpecParam2{
					Condition:      parsedAlert.Condition,
					Description:    alert.Description,
					Duration:       strconv.Itoa(parsedAlert.Duration),
					EscalationName: parsedAlert.EscalationName,
					File:           parsedAlert.File,
					Name:           alert.Name,
					Threshold:      parsedAlert.Threshold,
					Variable:       parsedAlert.Variable,
					VoteTag:        parsedAlert.VoteTag,
					VoteType:       parsedAlert.VoteType,
				}
				err := alertsUpdateLocator.Update(&params)
				if err != nil {
					return fmt.Errorf("Failed to update Alert %s: %s", alert.Name, err.Error())
				}
			}
		} else { // new alert
			fmt.Printf("  Adding Alert %s\n", alert.Name)
			params := cm15.AlertSpecParam{
				Condition:      parsedAlert.Condition,
				Description:    alert.Description,
				Duration:       strconv.Itoa(parsedAlert.Duration),
				EscalationName: parsedAlert.EscalationName,
				File:           parsedAlert.File,
				Name:           alert.Name,
				Threshold:      parsedAlert.Threshold,
				Variable:       parsedAlert.Variable,
				VoteTag:        parsedAlert.VoteTag,
				VoteType:       parsedAlert.VoteType,
			}
			_, err := alertsLocator.Create(&params)
			if err != nil {
				return fmt.Errorf("Failed to create Alert %s: %s", alert.Name, err.Error())
			}
		}
	}
	for _, alert := range existingAlerts {
		if !seenAlert[alert.Name] {
			fmt.Printf("  Removing alert %s\n", alert.Name)
			err := alert.Locator(client).Destroy()
			if err != nil {
				return fmt.Errorf("Could not destroy Alert %s: %s", alert.Name, err.Error())
			}
		}
	}

	return nil
}
