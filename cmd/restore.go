package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/hpcloud/cf-plugin-backup/models"
	"github.com/hpcloud/cf-plugin-backup/util"

	"github.com/spf13/cobra"
)

const (
	orgAudit     = "audited_organizations"
	orgBilling   = "billing_managed_organizations"
	orgManager   = "managed_organizations"
	orgDev       = "organizations"
	spaceAudit   = "audited_spaces"
	spaceManager = "managed_spaces"
	spaceDev     = "spaces"
)

type org struct {
	Name      string `json:"name"`
	QuotaGUID string `json:"quota_definition_guid"`
}

type space struct {
	Name             string `json:"name"`
	OrganizationGUID string `json:"organization_guid"`
}

type flag struct {
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	ErrorMessage string `json:"error_message,omitempty"`
	URL          string `json:"url"`
}

type app struct {
	Name               string      `json:"name"`
	SpaceGUID          string      `json:"space_guid"`
	Diego              interface{} `json:"diego"`
	Ports              interface{} `json:"ports"`
	Memory             interface{} `json:"memory"`
	Instances          interface{} `json:"instances"`
	DiskQuota          interface{} `json:"disk_quota"`
	StackGUID          string      `json:"stack_guid"`
	Command            interface{} `json:"command"`
	Buildpack          interface{} `json:"buildpack"`
	HealthCheckType    interface{} `json:"health_check_type"`
	HealthCheckTimeout interface{} `json:"health_check_timeout"`
	EnableSSH          interface{} `json:"enable_ssh"`
	DockerImage        interface{} `json:"docker_image"`
	EnvironmentJSON    interface{} `json:"environment_json"`
	State              interface{} `json:"state"`
}

type route struct {
	DomainGUID string      `json:"domain_guid"`
	SpaceGUID  string      `json:"space_guid"`
	Port       interface{} `json:"port"`
	Host       interface{} `json:"host"`
	Path       interface{} `json:"path"`
}

type securityGroup struct {
	Name       string      `json:"name"`
	Rules      interface{} `json:"rules"`
	SpaceGuids []string    `json:"space_guids"`
}

type sharedDomain struct {
	Name string `json:"name"`
}

type privateDomain struct {
	Name                   string `json:"name"`
	OwningOrganizationGUID string `json:"owning_organization_guid"`
}

func showInfo(sMessage string) {
	log.Printf(sMessage)
}

func showWarning(sMessage string) {
	log.Printf("WARNING: %s\n", sMessage)
}

func restorePrivateDomain(domain privateDomain) (string, error) {
	showInfo(fmt.Sprintf("Restoring private domain: %s", domain.Name))
	oJSON, err := json.Marshal(domain)
	util.FreakOut(err)

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/private_domains", "-H", "Content-Type: application/json",
		"-d", string(oJSON), "-X", "POST")
	if err != nil {
		showWarning(fmt.Sprintf("Could not create private domain %s, exception message: %s",
			domain.Name, err.Error()))
	}
	result, err := getResult(resp, "name", domain.Name)
	if err != nil {
		showWarning(fmt.Sprintf("Error restoring private domain %s: %s", domain.Name, err.Error()))
	} else {
		showInfo(fmt.Sprintf("Succesfully restored private domain %s", domain.Name))
	}
	return result, nil
}

func restoreUserRole(user, space, role string) {
	showInfo(fmt.Sprintf("Restoring role for User: %s", user))

	userID := getUserID(user)
	if userID == "" {
		showWarning(fmt.Sprintf("Could not find user: %s", user))
	} else {
		path := fmt.Sprintf("/v2/users/%s/%s/%s", userID, role, space)
		resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
			path, "-X", "PUT")
		if err != nil {
			showWarning(fmt.Sprintf("Could not create user association %s, exception message: %s",
				user, err.Error()))
		}
		_, err = getResult(resp, "", "")
		if err != nil {
			showWarning(fmt.Sprintf("Error restoring user role %s for user %s: %s", role, user, err.Error()))
		} else {
			showInfo(fmt.Sprintf("Succesfully restored user role %s for user %s", role, user))
		}
	}
}

func getUserID(user string) string {
	resources := util.GetResources(CliConnection, "/v2/users", 1)
	for _, u := range *resources {
		if u.Entity["username"] != nil {
			if u.Entity["username"].(string) == user {
				return u.Metadata["guid"].(string)
			}
		}
	}

	return ""
}

func restoreOrg(org org) string {
	showInfo(fmt.Sprintf("Restoring organization: %s", org.Name))
	oJSON, err := json.Marshal(org)
	util.FreakOut(err)

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/organizations", "-H", "Content-Type: application/json",
		"-d", string(oJSON), "-X", "POST")
	if err != nil {
		showWarning(fmt.Sprintf("Could not create organization %s, exception message: %s",
			org.Name, err.Error()))
	}
	result, err := getResult(resp, "name", org.Name)
	if err != nil {
		showWarning(fmt.Sprintf("Error restoring organization %s: %s", org.Name, err.Error()))
	} else {
		showInfo(fmt.Sprintf("Succesfully restored organization %s", org.Name))
	}
	return result
}

func restoreApp(app app) string {
	oJSON, err := json.Marshal(app)
	util.FreakOut(err)

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/apps", "-H", "Content-Type: application/json",
		"-d", string(oJSON), "-X", "POST")
	if err != nil {
		showWarning(fmt.Sprintf("Could not create application %s, exception message: %s",
			app.Name, err.Error()))
	}
	result, err := getResult(resp, "name", app.Name)
	if err != nil {
		showWarning(fmt.Sprintf("Error restoring application %s: %s", app.Name, err.Error()))
	} else {
		showInfo(fmt.Sprintf("Succesfully restored application %s", app.Name))
	}
	return result
}

func restoreFlag(flag models.FeatureFlagModel) string {
	showInfo(fmt.Sprintf("Restoring Flag: %s", flag.Name))

	var enabled string

	if flag.Enabled {
		enabled = "true"
	} else {
		enabled = "false"
	}

	pJSON := "{\"enabled\":" + enabled + "}"

	url := "/v2/config/feature_flags/" + flag.Name

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		url, "-H", "Content-Type: application/json",
		"-d", string(pJSON), "-X", "PUT")

	if err != nil {
		showWarning(fmt.Sprintf("Could not create flag %s, exception message: %s",
			flag.Name, err.Error()))
	}

	return showFlagResult(resp, flag)
}

func restoreSpace(space space) string {
	showInfo(fmt.Sprintf("Restoring space: %s", space.Name))
	oJSON, err := json.Marshal(space)
	util.FreakOut(err)

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/spaces", "-H", "Content-Type: application/json",
		"-d", string(oJSON), "-X", "POST")
	if err != nil {
		showWarning(fmt.Sprintf("Could not create space %s, exception message: %s",
			space.Name, err.Error()))
	}
	result, err := getResult(resp, "name", space.Name)
	if err != nil {
		showWarning(fmt.Sprintf("Error restoring space %s: %s", space.Name, err.Error()))
	} else {
		showInfo(fmt.Sprintf("Succesfully restored space %s", space.Name))
	}
	return result
}

func showFlagResult(resp []string, flag models.FeatureFlagModel) string {
	fResp := make(map[string]interface{})

	err := json.Unmarshal([]byte(resp[0]), &fResp)

	if err != nil {
		showWarning(fmt.Sprintf("Got unknow response while restoring flag %s: %s",
			flag.Name, err.Error()))
		return ""
	}

	if fResp["error_code"] != nil {
		showWarning(fmt.Sprintf("got %v while restoring flag %s",
			fResp["error_code"], flag.Name))
		return ""
	}

	if fResp["name"] != nil {
		inName := fResp["name"].(string)
		if inName == flag.Name {
			showInfo(fmt.Sprintf("Succesfully restored flag %s", flag.Name))
		} else {
			showWarning(fmt.Sprintf("Name %s does not match requested name %s",
				fResp["name"], flag.Name))
		}
	} else {
		showWarning(fmt.Sprintln("\tWarning unknown answer received"))
	}
	return ""
}

func getResult(resp []string, checkField, expectedValue string) (string, error) {
	oResp := make(map[string]interface{})
	if len(resp) == 0 {
		return "", fmt.Errorf("Got null response")
	}
	err := json.Unmarshal([]byte(util.ConcatStringArray(resp)), &oResp)
	if err != nil {
		return "", err
	}
	if oResp["error_code"] != nil {
		return "", fmt.Errorf("Got %v-%v", oResp["error_code"], oResp["description"])
	}

	if checkField != "" {
		if oResp["entity"] != nil {
			inName := (oResp["entity"].(map[string]interface{}))[checkField].(string)
			if inName == expectedValue {
				if oResp["metadata"] != nil {
					return (oResp["metadata"].(map[string]interface{}))["guid"].(string), nil
				}
			} else {
				return "", fmt.Errorf("Field %s does not match requested value %s", oResp[checkField], expectedValue)
			}
		} else {
			return "", fmt.Errorf("Warning unknown answer received")
		}
	}

	return "", nil
}

func restoreSharedDomain(sharedDomain sharedDomain) (string, error) {
	showInfo(fmt.Sprintf("Restoring shared domain: %s", sharedDomain.Name))
	oJSON, err := json.Marshal(sharedDomain)
	util.FreakOut(err)

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/shared_domains", "-H", "Content-Type: application/json",
		"-d", string(oJSON), "-X", "POST")
	if err != nil {
		showWarning(fmt.Sprintf("Could not create shared domain %s, exception message: %s",
			sharedDomain.Name, err.Error()))
	}
	result, err := getResult(resp, "name", sharedDomain.Name)
	if err != nil {
		showWarning(fmt.Sprintf("Error restoring shared domain %s: %s", sharedDomain.Name, err.Error()))
	} else {
		showInfo(fmt.Sprintf("Succesfully restored shared domain %s", sharedDomain.Name))
	}
	return result, nil
}

func restoreFromJSON(includeSecurityGroups bool) {

	//map["old_guid"] = "new_guid"
	spaceGuids := make(map[string]string)

	var fileContent []byte
	_, err := os.Stat(backupFile)
	util.FreakOut(err)

	fileContent, err = ioutil.ReadFile(backupFile)
	util.FreakOut(err)

	backupObject, err := util.ReadBackupJSON(fileContent)
	util.FreakOut(err)

	ccResources := util.CreateSharedDomainsCCResources(nil)
	sharedDomains := ccResources.TransformToResourceModels(backupObject.SharedDomains)

	for _, sd := range *sharedDomains {
		sharedDomain := sharedDomain{Name: sd.Entity["name"].(string)}
		restoreSharedDomain(sharedDomain)
	}

	orgs := util.RestoreOrgResourceModels(backupObject.Organizations)
	featureflags := util.RestoreFlagsResourceModels(backupObject.FeatureFlags)

	for _, flagobj := range *featureflags {
		restoreFlag(*flagobj)
	}

	for _, organization := range *orgs {
		o := org{Name: organization.Entity["name"].(string), QuotaGUID: organization.Entity["quota_definition_guid"].(string)}
		orgGUID := restoreOrg(o)

		if orgGUID != "" {
			if organization.Entity["auditors"] != nil {
				auditors := organization.Entity["auditors"].(*[]*models.ResourceModel)
				for _, auditor := range *auditors {
					restoreUserRole(auditor.Entity["username"].(string), orgGUID, orgDev)
					restoreUserRole(auditor.Entity["username"].(string), orgGUID, orgAudit)
				}
			}

			if organization.Entity["billing_managers"] != nil {
				billingManagers := organization.Entity["billing_managers"].(*[]*models.ResourceModel)
				for _, manager := range *billingManagers {
					restoreUserRole(manager.Entity["username"].(string), orgGUID, orgDev)
					restoreUserRole(manager.Entity["username"].(string), orgGUID, orgBilling)
				}
			}

			if organization.Entity["managers"] != nil {
				managers := organization.Entity["managers"].(*[]*models.ResourceModel)
				for _, manager := range *managers {
					restoreUserRole(manager.Entity["username"].(string), orgGUID, orgDev)
					restoreUserRole(manager.Entity["username"].(string), orgGUID, orgManager)
				}
			}

			if organization.Entity["private_domains"] != nil {
				privateDomains := organization.Entity["private_domains"].(*[]*models.ResourceModel)
				for _, domain := range *privateDomains {
					pd := privateDomain{Name: domain.Entity["name"].(string), OwningOrganizationGUID: orgGUID}
					restorePrivateDomain(pd)
				}
			}

			spaces := organization.Entity["spaces"].(*[]*models.ResourceModel)
			for _, sp := range *spaces {
				s := space{Name: sp.Entity["name"].(string), OrganizationGUID: orgGUID}
				spaceGUID := restoreSpace(s)
				spaceGuids[sp.Metadata["guid"].(string)] = spaceGUID

				if spaceGUID != "" {
					if sp.Entity["auditors"] != nil {
						auditors := sp.Entity["auditors"].(*[]*models.ResourceModel)
						for _, auditor := range *auditors {
							restoreUserRole(auditor.Entity["username"].(string), spaceGUID, spaceAudit)
						}
					}

					if sp.Entity["developers"] != nil {
						developers := sp.Entity["developers"].(*[]*models.ResourceModel)
						for _, developer := range *developers {
							restoreUserRole(developer.Entity["username"].(string), spaceGUID, spaceDev)
						}
					}

					if sp.Entity["managers"] != nil {
						managers := sp.Entity["managers"].(*[]*models.ResourceModel)
						for _, manager := range *managers {
							restoreUserRole(manager.Entity["username"].(string), spaceGUID, spaceManager)
						}
					}
				}

				//continue if there are no apps to restore
				if sp.Entity["apps"] == nil {
					continue
				}

				apps := sp.Entity["apps"].(*[]*models.ResourceModel)
				packager := &util.CFPackager{
					Cli:    CliConnection,
					Writer: new(util.CFFileWriter),
					Reader: new(util.CFFileReader),
				}
				appBits := util.NewCFDroplet(CliConnection, packager)

				appsCount := len(*apps)
				appIndex := 1

				for _, application := range *apps {
					stackName := application.Entity["stack"].(*models.ResourceModel).Entity["name"].(string)
					stackGUID := getStackGUID(stackName)
					if stackGUID == "" {
						showWarning(fmt.Sprintf("Stack %s not found. Skipping app %s", stackName, application.Entity["name"].(string)))
						continue
					}

					a := app{
						Name:               application.Entity["name"].(string),
						SpaceGUID:          spaceGUID,
						Diego:              application.Entity["diego"],
						Memory:             application.Entity["memory"],
						Instances:          application.Entity["instances"],
						DiskQuota:          application.Entity["disk_quota"],
						StackGUID:          stackGUID,
						Command:            application.Entity["command"],
						Buildpack:          application.Entity["buildpack"],
						HealthCheckType:    application.Entity["health_check_type"],
						HealthCheckTimeout: application.Entity["health_check_timeout"],
						EnableSSH:          application.Entity["enable_ssh"],
						DockerImage:        application.Entity["docker_image"],
						EnvironmentJSON:    application.Entity["environment_json"],
						Ports:              application.Entity["ports"],
					}

					showInfo(fmt.Sprintf("Restoring App %s for space %s [%d/%d]", a.Name, sp.Entity["name"].(string), appIndex, appsCount))

					appGUID := restoreApp(a)

					if dockerImg, hit := application.Entity["docker_image"]; !hit || dockerImg == nil {
						oldAppGUID := application.Metadata["guid"].(string)
						appZipPath := filepath.Join(backupDir, backupAppBitsDir, oldAppGUID+".zip")
						err = appBits.UploadDroplet(appGUID, appZipPath)
						if err != nil {
							showWarning(fmt.Sprintf("Could not upload app bits for app %s: %s", application.Entity["name"].(string), err.Error()))
						}
					}

					state := application.Entity["state"].(string)
					a.State = state
					updateApp(appGUID, a)

					boundRoute := false
					if application.Entity["routes"] != nil {
						routes := application.Entity["routes"].(*[]*models.ResourceModel)
						for _, rt := range *routes {
							domain := rt.Entity["domain"].(*models.ResourceModel)

							r := route{
								SpaceGUID: spaceGUID,
								Port:      rt.Entity["port"],
								Path:      rt.Entity["path"],
								Host:      rt.Entity["host"],
							}

							domainName := domain.Entity["name"].(string)

							if domain.Entity["owning_organization_guid"] == nil {
								domainGUID := getSharedDomainGUID(domainName)
								if domainGUID == "" {
									showWarning(fmt.Sprintf("Could not find shared domain %s", domainName))
									continue
								}
								r.DomainGUID = domainGUID
							} else {
								domainGUID := getPrivateDomainGUID(domainName)
								if domainGUID == "" {
									showWarning(fmt.Sprintf("Could not find private domain %s", domainName))
									continue
								}
								r.DomainGUID = domainGUID
							}
							routeGUID := createRoute(r)
							showInfo(fmt.Sprintf("Binding route %s.%s to app %s", r.Host, domainName, a.Name))
							err = bindRoute(appGUID, routeGUID)
							if err != nil {
								showWarning(fmt.Sprintf("Error binding route %s.%s to app %s: %s", r.Host, domainName, a.Name, err.Error()))
							} else {
								boundRoute = true
								showInfo(fmt.Sprintf("Successfully bound route %s.%s to app %s", r.Host, domainName, a.Name))
							}
						}
					}

					if !boundRoute {
						domain := getFirstSharedDomainGUID()
						if domain == nil {
							showWarning(fmt.Sprintf("Could not find any shared domain for app %s.", a.Name))
						} else {
							r := route{
								SpaceGUID:  spaceGUID,
								Host:       appGUID,
								DomainGUID: domain.Metadata["guid"].(string),
							}
							routeGUID := createRoute(r)
							showInfo(fmt.Sprintf("Binding new route to app %s", a.Name))
							err = bindRoute(appGUID, routeGUID)
							if err != nil {
								showWarning(fmt.Sprintf("Error binding new route to app %s: %s", a.Name, err.Error()))
							} else {
								showInfo(fmt.Sprintf("Successfully bound new route to app %s", a.Name))
							}

						}
					}
					appIndex++
				}
			}
		}
	}

	if includeSecurityGroups {
		ccResources := util.CreateSecurityGroupsCCResources(nil)
		securityGroups := ccResources.TransformToResourceModels(backupObject.SecurityGroups)
		for _, sg := range *securityGroups {
			spaces := *sg.Entity["spaces"].(*[]*models.ResourceModel)
			newSpaces := make([]string, len(spaces))
			for i, s := range spaces {
				newSpaces[i] = spaceGuids[(s.Metadata["guid"]).(string)]
			}

			g := securityGroup{
				Name:       sg.Entity["name"].(string),
				Rules:      sg.Entity["rules"],
				SpaceGuids: newSpaces,
			}

			_, err = restoreSecurityGroup(g)
			if err != nil {
				showWarning(fmt.Sprintf("Error restoring security group %s: %s", g.Name, err.Error()))
			}
		}
	}
}

func restoreSecurityGroup(securityGroup securityGroup) (string, error) {
	showInfo(fmt.Sprintf("Restoring security group %s", securityGroup.Name))
	resources := util.GetResources(CliConnection, "/v2/security_groups?q=name:"+securityGroup.Name, 1)
	for _, u := range *resources {
		if u.Entity["name"].(string) == securityGroup.Name {
			showInfo(fmt.Sprintf("Deleting old security group %s", securityGroup.Name))
			err := deleteSecurityGroup(u.Metadata["guid"].(string))
			if err != nil {
				return "", err
			}
			break
		}
	}
	return createSecurityGroup(securityGroup)
}

func deleteSecurityGroup(guid string) error {
	_, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/security_groups/"+guid, "-H", "Content-Type: application/x-www-form-urlencoded",
		"-X", "DELETE")
	if err != nil {
		return err
	}

	return nil
}

func createSecurityGroup(securityGroup securityGroup) (string, error) {
	showInfo(fmt.Sprintf("Creating security group: %s", securityGroup.Name))
	oJSON, err := json.Marshal(securityGroup)
	util.FreakOut(err)

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/security_groups", "-H", "Content-Type: application/json",
		"-d", string(oJSON), "-X", "POST")
	if err != nil {
		showWarning(fmt.Sprintf("Could not create security group %s, exception message: %s",
			securityGroup.Name, err.Error()))
	}
	result, err := getResult(resp, "name", securityGroup.Name)
	if err != nil {
		showWarning(fmt.Sprintf("Error restoring security group %s: %s", securityGroup.Name, err.Error()))
	} else {
		showInfo(fmt.Sprintf("Succesfully restored security group %s", securityGroup.Name))
	}
	return result, nil
}

func bindRoute(appGUID, routeGUID string) error {
	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/apps/"+appGUID+"/routes/"+routeGUID, "-H", "Content-Type: application/x-www-form-urlencoded",
		"-X", "PUT")
	if err != nil {
		return err
	}
	_, err = getResult(resp, "", "")
	if err != nil {
		return err
	}
	return nil
}

func createRoute(route route) string {
	showInfo(fmt.Sprintf("Creating route: %s", route.Host))
	oJSON, err := json.Marshal(route)
	util.FreakOut(err)

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/routes", "-H", "Content-Type: application/json",
		"-d", string(oJSON), "-X", "POST")
	if err != nil {
		showWarning(fmt.Sprintf("Could not create route %s, exception message: %s",
			route.Host, err.Error()))
	}
	result, err := getResult(resp, "host", route.Host.(string))
	if err != nil {
		showWarning(fmt.Sprintf("Error creating route %s: %s", route.Host, err.Error()))
	} else {
		showInfo(fmt.Sprintf("Succesfully created route %s", route.Host))
	}
	return result
}

func getFirstSharedDomainGUID() *models.ResourceModel {
	resources := util.GetResources(CliConnection, "/v2/shared_domains", 1)
	if len(*resources) > 0 {
		return (*resources)[0]
	}

	return nil
}

func getSharedDomainGUID(domainName string) string {
	resources := util.GetResources(CliConnection, "/v2/shared_domains?q=name:"+domainName, 1)
	for _, u := range *resources {
		if u.Entity["name"].(string) == domainName {
			return u.Metadata["guid"].(string)
		}
	}

	return ""
}

func getPrivateDomainGUID(domainName string) string {
	resources := util.GetResources(CliConnection, "/v2/private_domains?q=name:"+domainName, 1)
	for _, u := range *resources {
		if u.Entity["name"].(string) == domainName {
			return u.Metadata["guid"].(string)
		}
	}

	return ""
}

func getStackGUID(stackName string) string {
	resources := util.GetResources(CliConnection, "/v2/stacks?q=name:"+stackName, 1)
	for _, u := range *resources {
		if u.Entity["name"].(string) == stackName {
			return u.Metadata["guid"].(string)
		}
	}

	return ""
}

func updateApp(guid string, app app) {
	showInfo(fmt.Sprintf("Updating app %s", app.Name))
	oJSON, err := json.Marshal(app)
	util.FreakOut(err)

	resp, err := CliConnection.CliCommandWithoutTerminalOutput("curl",
		"/v2/apps/"+guid, "-H", "Content-Type: application/json",
		"-d", string(oJSON), "-X", "PUT")
	if err != nil {
		showWarning(fmt.Sprintf("Could not update app %s, exception message: %s",
			app.Name, err.Error()))
	}
	_, err = getResult(resp, "name", app.Name)
	if err != nil {
		showWarning(fmt.Sprintf("Error updating application %s: %s", app.Name, err.Error()))
	} else {
		showInfo(fmt.Sprintf("Succesfully updated application %s", app.Name))
	}
}

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore the CloudFoundry state from a backup",
	Long: `Restore the CloudFoundry state from a backup created with the snapshot command
`,
	Run: func(cmd *cobra.Command, args []string) {
		includeSecurityGroups, _ := cmd.Flags().GetBool("include-security-groups")
		restoreFromJSON(includeSecurityGroups)
	},
}

func init() {
	restoreCmd.Flags().Bool("include-security-groups", false, "Restore security groups")
	RootCmd.AddCommand(restoreCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// restoreCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// restoreCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}
