/*
 * Copyright 2015-2016 IBM Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package commands

import (
  "encoding/base64"
  "errors"
  "fmt"
  "os/exec"
  "path/filepath"
  "strings"

  "../../go-whisk/whisk"
  "../wski18n"

  "github.com/fatih/color"
  "github.com/spf13/cobra"
  "github.com/mattn/go-colorable"
)

const MEMORY_LIMIT = 256
const TIMEOUT_LIMIT = 60000
const LOGSIZE_LIMIT = 10

//////////////
// Commands //
//////////////

var actionCmd = &cobra.Command{
  Use:   "action",
  Short: wski18n.T("work with actions"),
}

var actionCreateCmd = &cobra.Command{
  Use:           "create ACTION_NAME ACTION",
  Short:         wski18n.T("create a new action"),
  SilenceUsage:  true,
  SilenceErrors: true,
  PreRunE:       setupClientConfig,
  RunE: func(cmd *cobra.Command, args []string) error {

    if whiskErr := checkArgs(args, 2, 2, "Action create",
            wski18n.T("An action name and action are required.")); whiskErr != nil {
      return whiskErr
    }

    action, sharedSet, err := parseAction(cmd, args)
    if err != nil {
      whisk.Debug(whisk.DbgError, "parseAction(%s, %s) error: %s\n", cmd, args, err)
      errMsg := fmt.Sprintf(
        wski18n.T("Unable to parse action command arguments: {{.err}}",
          map[string]interface{}{"err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
      return whiskErr
    }

    _, _, err = client.Actions.Insert(action, sharedSet, false)
    if err != nil {
      whisk.Debug(whisk.DbgError, "client.Actions.Insert(%#v, %t, false) error: %s\n", action, sharedSet, err)
      errMsg := fmt.Sprintf(
        wski18n.T("Unable to create action: {{.err}}",
          map[string]interface{}{"err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_NETWORK,
        whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
      return whiskErr
    }

    fmt.Fprintf(color.Output,
      wski18n.T("{{.ok}} created action {{.name}}\n",
        map[string]interface{}{"ok": color.GreenString("ok:"), "name": boldString(action.Name)}))
    return nil
  },
}

var actionUpdateCmd = &cobra.Command{
  Use:           "update ACTION_NAME [ACTION]",
  Short:         wski18n.T("update an existing action"),
  SilenceUsage:  true,
  SilenceErrors: true,
  PreRunE:       setupClientConfig,
  RunE: func(cmd *cobra.Command, args []string) error {

    if whiskErr := checkArgs(args, 1, 2, "Action update",
        wski18n.T("An action name is required. An action is optional.")); whiskErr != nil {
      return whiskErr
    }

    action, sharedSet, err := parseAction(cmd, args)
    if err != nil {
      whisk.Debug(whisk.DbgError, "parseAction(%s, %s) error: %s\n", cmd, args, err)
      errMsg := fmt.Sprintf(
        wski18n.T("Unable to parse action command arguments: {{.err}}",
          map[string]interface{}{"err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
      return whiskErr
    }

    _, _, err = client.Actions.Insert(action, sharedSet, true)
    if err != nil {
      whisk.Debug(whisk.DbgError, "client.Actions.Insert(%#v, %t, false) error: %s\n", action, sharedSet, err)
      errMsg := fmt.Sprintf(
        wski18n.T("Unable to update action: {{.err}}",
          map[string]interface{}{"err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_NETWORK,
        whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
      return whiskErr
    }

    fmt.Fprintf(color.Output,
      wski18n.T("{{.ok}} updated action {{.name}}\n",
        map[string]interface{}{"ok": color.GreenString("ok:"), "name": boldString(action.Name)}))
    return nil
  },
}

var actionInvokeCmd = &cobra.Command{
  Use:           "invoke ACTION_NAME",
  Short:         wski18n.T("invoke action"),
  SilenceUsage:  true,
  SilenceErrors: true,
  PreRunE:       setupClientConfig,
  RunE: func(cmd *cobra.Command, args []string) error {
    var err error
    var parameters interface{}

    if whiskErr := checkArgs(args, 1, 1, "Action invoke", wski18n.T("An action name is required.")); whiskErr != nil {
      return whiskErr
    }

    qName, err := parseQualifiedName(args[0])
    if err != nil {
      whisk.Debug(whisk.DbgError, "parseQualifiedName(%s) failed: %s\n", args[0], err)
      errMsg := fmt.Sprintf(
        wski18n.T("'{{.name}}' is not a valid qualified name: {{.err}}",
          map[string]interface{}{"name": args[0], "err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
      return whiskErr
    }
    client.Namespace = qName.namespace

    if len(flags.common.param) > 0 {
      whisk.Debug(whisk.DbgInfo, "Parsing parameters: %#v\n", flags.common.param)

      parameters, err = getJSONFromStrings(flags.common.param, false)
      if err != nil {
        whisk.Debug(whisk.DbgError, "getJSONFromStrings(%#v, false) failed: %s\n", flags.common.param, err)
        errMsg := fmt.Sprintf(
          wski18n.T("Invalid parameter argument '{{.param}}': {{.err}}",
            map[string]interface{}{"param": fmt.Sprintf("%#v", flags.common.param), "err": err}))
        whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
          whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
        return whiskErr
      }

    }

    outputStream := color.Output

    activation, _, err := client.Actions.Invoke(qName.entityName, parameters, flags.common.blocking)
    if err != nil {
      whiskErr, isWhiskErr := err.(*whisk.WskError)

      if (isWhiskErr && whiskErr.ApplicationError != true) || !isWhiskErr {
        whisk.Debug(whisk.DbgError, "client.Actions.Invoke(%s, %s, %t) error: %s\n", qName.entityName, parameters,
          flags.common.blocking, err)
        errMsg := fmt.Sprintf(
          wski18n.T("Unable to invoke action '{{.name}}': {{.err}}",
            map[string]interface{}{"name": qName.entityName, "err": err}))
        whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
          whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
        return whiskErr
      } else {
        outputStream = colorable.NewColorableStderr()
      }
    }

    if flags.common.blocking && flags.action.result {
      printJSON(activation.Response.Result, outputStream)
    } else if flags.common.blocking {
      fmt.Fprintf(color.Output,
        wski18n.T("{{.ok}} invoked /{{.namespace}}/{{.name}} with id {{.id}}\n",
          map[string]interface{}{
            "ok": color.GreenString("ok:"),
            "namespace": boldString(qName.namespace),
            "name": boldString(qName.entityName),
            "id": boldString(activation.ActivationID)}))
      printJSON(activation, outputStream)
    } else {
      fmt.Fprintf(color.Output,
        wski18n.T("{{.ok}} invoked /{{.namespace}}/{{.name}} with id {{.id}}\n",
          map[string]interface{}{
            "ok": color.GreenString("ok:"),
            "namespace": boldString(qName.namespace),
            "name": boldString(qName.entityName),
            "id": boldString(activation.ActivationID)}))
    }

    return err
  },
}

var actionGetCmd = &cobra.Command{
  Use:           "get ACTION_NAME",
  Short:         wski18n.T("get action"),
  SilenceUsage:  true,
  SilenceErrors: true,
  PreRunE:       setupClientConfig,
  RunE: func(cmd *cobra.Command, args []string) error {
    var err error

    if whiskErr := checkArgs(args, 1, 1, "Action get", wski18n.T("An action name is required.")); whiskErr != nil {
      return whiskErr
    }

    qName, err := parseQualifiedName(args[0])
    if err != nil {
      whisk.Debug(whisk.DbgError, "parseQualifiedName(%s) failed: %s\n", args[0], err)
      errMsg := fmt.Sprintf(
        wski18n.T("'{{.name}}' is not a valid qualified name: {{.err}}",
          map[string]interface{}{"name": args[0], "err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
      return whiskErr
    }
    client.Namespace = qName.namespace

    action, _, err := client.Actions.Get(qName.entityName)
    if err != nil {
      whisk.Debug(whisk.DbgError, "client.Actions.Get(%s) error: %s\n", qName.entityName, err)
      errMsg := fmt.Sprintf(
        wski18n.T("Unable to get action: {{.err}}",
          map[string]interface{}{"err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
      return whiskErr
    }

    if flags.common.summary {
      printSummary(action)
    } else {
      fmt.Fprintf(color.Output,
        wski18n.T("{{.ok}} got action {{.name}}\n",
          map[string]interface{}{"ok": color.GreenString("ok:"), "name": boldString(qName.entityName)}))
      printJSON(action)
    }

    return nil
  },
}

var actionDeleteCmd = &cobra.Command{
  Use:           "delete ACTION_NAME",
  Short:         wski18n.T("delete action"),
  SilenceUsage:  true,
  SilenceErrors: true,
  PreRunE:       setupClientConfig,
  RunE: func(cmd *cobra.Command, args []string) error {

    if whiskErr := checkArgs(args, 1, 1, "Action delete", wski18n.T("An action name is required.")); whiskErr != nil {
      return whiskErr
    }

    qName, err := parseQualifiedName(args[0])
    if err != nil {
      whisk.Debug(whisk.DbgError, "parseQualifiedName(%s) failed: %s\n", args[0], err)
      errMsg := fmt.Sprintf(
        wski18n.T("'{{.name}}' is not a valid qualified name: {{.err}}",
          map[string]interface{}{"name": args[0], "err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
      return whiskErr
    }
    client.Namespace = qName.namespace

    _, err = client.Actions.Delete(qName.entityName)
    if err != nil {
      whisk.Debug(whisk.DbgError, "client.Actions.Delete(%s) error: %s\n", qName.entityName, err)
      errMsg := fmt.Sprintf(
        wski18n.T("Unable to delete action: {{.err}}",
          map[string]interface{}{"err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
      return whiskErr
    }

    fmt.Fprintf(color.Output,
      wski18n.T("{{.ok}} deleted action {{.name}}\n",
        map[string]interface{}{"ok": color.GreenString("ok:"), "name": boldString(qName.entityName)}))
    return nil
  },
}

var actionListCmd = &cobra.Command{
  Use:           "list [NAMESPACE]",
  Short:         wski18n.T("list all actions"),
  SilenceUsage:  true,
  SilenceErrors: true,
  PreRunE:       setupClientConfig,
  RunE: func(cmd *cobra.Command, args []string) error {
    var qName qualifiedName
    var err error

    if len(args) == 1 {
      qName, err = parseQualifiedName(args[0])
      if err != nil {
        whisk.Debug(whisk.DbgError, "parseQualifiedName(%s) failed: %s\n", args[0], err)
        errMsg := fmt.Sprintf(
          wski18n.T("'{{.name}}' is not a valid qualified name: {{.err}}",
            map[string]interface{}{"name": args[0], "err": err}))
        whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
          whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
        return whiskErr
      }

      if len(qName.namespace) == 0 {
        whisk.Debug(whisk.DbgError, "Namespace is missing from '%s'\n", args[0])
        errMsg := fmt.Sprintf(
          wski18n.T("No valid namespace detected. Run 'wsk property set --namespace' or ensure the name argument is preceded by a \"/\""))
        whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
          whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
        return whiskErr
      }
      client.Namespace = qName.namespace
    } else if whiskErr := checkArgs(args, 0, 1, "Action list",
        wski18n.T("An optional namespace is the only valid argument.")); whiskErr != nil {
      return whiskErr
    }

    options := &whisk.ActionListOptions{
      Skip:  flags.common.skip,
      Limit: flags.common.limit,
    }

    actions, _, err := client.Actions.List(qName.entityName, options)
    if err != nil {
      whisk.Debug(whisk.DbgError, "client.Actions.List(%s, %#v) error: %s\n", qName.entityName, options, err)
      errMsg := wski18n.T("Unable to obtain the list of actions for namespace '{{.name}}': {{.err}}",
          map[string]interface{}{"name": getClientNamespace(), "err": err})
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_NETWORK,
        whisk.DISPLAY_MSG, whisk.NO_DISPLAY_USAGE)
      return whiskErr
    }

    printList(actions)
    return nil
  },
}

func getJavaClasses(classes []string) []string {
  var res []string

  for i := 0; i < len(classes); i++ {
    if strings.HasSuffix(classes[i], ".class") {
      classes[i] = classes[i][0 : len(classes[i])-6]
      classes[i] = strings.Replace(classes[i], "/", ".", -1)
      res = append(res, classes[i])
    }
  }

  return res
}

func findMainJarClass(jarFile string) (string, error) {
  signature := "public static com.google.gson.JsonObject main(com.google.gson.JsonObject);"

  whisk.Debug(whisk.DbgInfo, "unjaring '%s'\n", jarFile)
  stdOut, err := exec.Command("jar", "-tf", jarFile).Output()
  if err != nil {
    whisk.Debug(whisk.DbgError, "unjar of '%s' failed: %s\n", jarFile, err)
    return "", err
  }

  whisk.Debug(whisk.DbgInfo, "jar stdout:\n%s\n", stdOut)
  output := string(stdOut[:])
  output = strings.Replace(output, "\r", "", -1) // Windows jar adds \r chars that needs removing
  outputArr := strings.Split(output, "\n")
  classes := getJavaClasses(outputArr)

  whisk.Debug(whisk.DbgInfo, "jar '%s' has %d classes\n", jarFile, len(classes))
  for i := 0; i < len(classes); i++ {
    whisk.Debug(whisk.DbgInfo, "javap -public -cp '%s'\n", classes[i])
    stdOut, err = exec.Command("javap", "-public", "-cp", jarFile, classes[i]).Output()
    if err != nil {
      whisk.Debug(whisk.DbgError, "javap of class '%s' in jar '%s' failed: %s\n", classes[i], jarFile, err)
      return "", err
    }

    output := string(stdOut[:])
    whisk.Debug(whisk.DbgInfo, "javap '%s' output:\n%s\n", classes[i], output)

    if strings.Contains(output, signature) {
      return classes[i], nil
    }
  }

  errMsg := fmt.Sprintf(
    wski18n.T("Could not find 'main' method in '{{.name}}'",
      map[string]interface{}{"name": jarFile}))
  whiskErr := whisk.MakeWskError(errors.New(errMsg), whisk.EXITCODE_ERR_GENERAL,
    whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)

  return "", whiskErr
}

func parseAction(cmd *cobra.Command, args []string) (*whisk.Action, bool, error) {
  var err error
  var shared, sharedSet bool
  var artifact string

  qName := qualifiedName{}
  qName, err = parseQualifiedName(args[0])
  if err != nil {
    whisk.Debug(whisk.DbgError, "parseQualifiedName(%s) failed: %s\n", args[0], err)
    errMsg := fmt.Sprintf(
      wski18n.T("'{{.name}}' is not a valid qualified name: {{.err}}",
        map[string]interface{}{"name": args[0], "err": err}))
    whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
      whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
    return nil, sharedSet, whiskErr
  }
  client.Namespace = qName.namespace

  if len(args) == 2 {
    artifact = args[1]
  }

  if flags.action.shared == "yes" {
    shared = true
    sharedSet = true
  } else if flags.action.shared == "no" {
    shared = false
    sharedSet = true
  } else {
    sharedSet = false
  }

  whisk.Debug(whisk.DbgInfo, "Parsing parameters: %#v\n", flags.common.param)
  parameters, err := getJSONFromStrings(flags.common.param, true)
  if err != nil {
    whisk.Debug(whisk.DbgError, "getJSONFromStrings(%#v, true) failed: %s\n", flags.common.param, err)
    errMsg := fmt.Sprintf(
      wski18n.T("Invalid parameter argument '{{.param}}': {{.err}}",
        map[string]interface{}{"param": fmt.Sprintf("%#v", flags.common.param), "err": err}))
    whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
      whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
    return nil, sharedSet, whiskErr
  }

  whisk.Debug(whisk.DbgInfo, "Parsing annotations: %#v\n", flags.common.annotation)
  annotations, err := getJSONFromStrings(flags.common.annotation, true)
  if err != nil {
    whisk.Debug(whisk.DbgError, "getJSONFromStrings(%#v, true) failed: %s\n", flags.common.annotation, err)
    errMsg := fmt.Sprintf(
      wski18n.T("Invalid annotation argument '{{.annotation}}': {{.err}}",
        map[string]interface{}{"annotation": fmt.Sprintf("%#v", flags.common.annotation), "err": err}))
    whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
      whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
    return nil, sharedSet, whiskErr
  }

  action := new(whisk.Action)

  if flags.action.docker {
    action.Exec = new(whisk.Exec)
    action.Exec.Image = artifact
    action.Exec.Kind = "blackbox"
  } else if flags.action.copy {
    qNameCopy := qualifiedName{}
    qNameCopy, err = parseQualifiedName(args[1])
    if err != nil {
      whisk.Debug(whisk.DbgError, "parseQualifiedName(%s) failed: %s\n", args[1], err)
      errMsg := fmt.Sprintf(
        wski18n.T("'{{.name}}' is not a valid qualified name: {{.err}}",
          map[string]interface{}{"name": args[1], "err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
      return nil, sharedSet, whiskErr
    }
    client.Namespace = qNameCopy.namespace

    existingAction, _, err := client.Actions.Get(qNameCopy.entityName)
    if err != nil {
      whisk.Debug(whisk.DbgError, "client.Actions.Get(%s) error: %s\n", qName.entityName, err)
      errMsg := fmt.Sprintf(
        wski18n.T("Unable to obtain action '{{.name}}' to copy: {{.err}}",
          map[string]interface{}{"name": qName.entityName, "err": err}))
      whiskErr := whisk.MakeWskErrorFromWskError(errors.New(errMsg), err, whisk.EXITCODE_ERR_GENERAL,
        whisk.DISPLAY_MSG, whisk.DISPLAY_USAGE)
      return nil, sharedSet, whiskErr
    }

    client.Namespace = qName.namespace
    action.Exec = existingAction.Exec
  } else if flags.action.sequence {
    action.Exec = new(whisk.Exec)
    action.Exec.Kind = "sequence"
    action.Exec.Components = csvToQualifiedActions(artifact)
  } else if artifact != "" {
    ext := filepath.Ext(artifact)
    action.Exec = new(whisk.Exec)
    action.Exec.Code, err = readFile(artifact)

    if err != nil {
      whisk.Debug(whisk.DbgError, "readFile(%s) error: %s\n", artifact, err)
      return nil, sharedSet, err
    }

    if flags.action.kind == "swift:3" || flags.action.kind == "swift:3.0" || flags.action.kind == "swift:3.0.0" {
      action.Exec.Kind = "swift:3"
    } else if flags.action.kind == "nodejs:6" || flags.action.kind == "nodejs:6.0" || flags.action.kind == "nodejs:6.0.0" {
      action.Exec.Kind = "nodejs:6"
    } else if flags.action.kind == "nodejs:default" {
      action.Exec.Kind = "nodejs:default"
    } else if flags.action.kind == "nodejs" {
      action.Exec.Kind = "nodejs"
    } else if flags.action.kind == "swift" {
      action.Exec.Kind = "swift"
    } else if len(flags.action.kind) > 0 {
      whisk.Debug(whisk.DbgError, "--kind argument '%s' is not supported\n", flags.action.kind)
      errMsg := fmt.Sprintf(
        wski18n.T("'{{.name}}' is not a supported action runtime",
          map[string]interface{}{"name": flags.action.kind}))
      whiskErr := whisk.MakeWskError(errors.New(errMsg), whisk.EXITCODE_ERR_GENERAL, whisk.DISPLAY_MSG,
        whisk.DISPLAY_USAGE)
      return nil, sharedSet, whiskErr
    } else if ext == ".swift" {
      action.Exec.Kind = "swift"
    } else if ext == ".js" {
      action.Exec.Kind = "nodejs:6"
    } else if ext == ".py" {
      action.Exec.Kind = "python"
    } else if ext == ".jar" {
      action.Exec.Kind = "java"
      action.Exec.Jar = base64.StdEncoding.EncodeToString([]byte(action.Exec.Code))
      action.Exec.Main, err = findMainJarClass(artifact)
      action.Exec.Code = ""

      if err != nil {
        return nil, sharedSet, err
      }
    } else {
      whisk.Debug(whisk.DbgError, "Action runtime extension '%s' is not supported\n", ext)
      errMsg := fmt.Sprintf(
        wski18n.T("'{{.name}}' is not a supported action runtime",
          map[string]interface{}{"name": ext}))
      whiskErr := whisk.MakeWskError(errors.New(errMsg), whisk.EXITCODE_ERR_GENERAL, whisk.DISPLAY_MSG,
        whisk.DISPLAY_USAGE)
      return nil, sharedSet, whiskErr
    }
  }

  action.Name = qName.entityName
  action.Namespace = qName.namespace
  action.Publish = shared
  action.Annotations = annotations.(whisk.KeyValueArr)
  action.Limits = getLimits()

  // If the action sequence is not already the Parameters value, set it to the --param parameter values
  if action.Parameters == nil && parameters != nil {
    action.Parameters = parameters.(whisk.KeyValueArr)
  }

  whisk.Debug(whisk.DbgInfo, "Parsed action struct: %#v\n", action)
  return action, sharedSet, nil
}

func getLimits() (*whisk.Limits) {
  var limits *whisk.Limits

  if flags.action.memory != MEMORY_LIMIT || flags.action.timeout != TIMEOUT_LIMIT ||
    flags.action.logsize != LOGSIZE_LIMIT {
    limits = new(whisk.Limits)

    if flags.action.memory != MEMORY_LIMIT {
      limits.Memory = &flags.action.memory
    }

    if flags.action.timeout != TIMEOUT_LIMIT {
      limits.Timeout = &flags.action.timeout
    }

    if flags.action.logsize != LOGSIZE_LIMIT {
      limits.Logsize = &flags.action.logsize
    }

    whisk.Debug(whisk.DbgInfo, "Action limits: %+v\n", limits)
  }

  return limits
}

///////////
// Flags //
///////////

func init() {
  actionCreateCmd.Flags().BoolVar(&flags.action.docker, "docker", false, wski18n.T("treat ACTION as docker image path on dockerhub"))
  actionCreateCmd.Flags().BoolVar(&flags.action.copy, "copy", false, wski18n.T("treat ACTION as the name of an existing action"))
  actionCreateCmd.Flags().BoolVar(&flags.action.sequence, "sequence", false, wski18n.T("treat ACTION as comma separated sequence of actions to invoke"))
  actionCreateCmd.Flags().StringVar(&flags.action.kind, "kind", "", wski18n.T("the `KIND` of the action runtime (example: swift:3, nodejs:6)"))
  actionCreateCmd.Flags().StringVar(&flags.action.shared, "shared", "no", wski18n.T("action visibility `SCOPE`; yes = shared, no = private"))
  actionCreateCmd.Flags().IntVarP(&flags.action.timeout, "timeout", "t", TIMEOUT_LIMIT, wski18n.T("the timeout `LIMIT` in milliseconds after which the action is terminated"))
  actionCreateCmd.Flags().IntVarP(&flags.action.memory, "memory", "m", MEMORY_LIMIT, wski18n.T("the maximum memory `LIMIT` in MB for the action"))
  actionCreateCmd.Flags().IntVarP(&flags.action.logsize, "logsize", "l", LOGSIZE_LIMIT, wski18n.T("the maximum log size `LIMIT` in MB for the action"))
  actionCreateCmd.Flags().StringSliceVarP(&flags.common.annotation, "annotation", "a", nil, wski18n.T("annotation values in `KEY VALUE` format"))
  actionCreateCmd.Flags().StringVarP(&flags.common.annotFile, "annotation-file", "A", "", wski18n.T("`FILE` containing annotation values in JSON format"))
  actionCreateCmd.Flags().StringSliceVarP(&flags.common.param, "param", "p", nil, wski18n.T("parameter values in `KEY VALUE` format"))
  actionCreateCmd.Flags().StringVarP(&flags.common.paramFile, "param-file", "P", "", wski18n.T("`FILE` containing parameter values in JSON format"))

  actionUpdateCmd.Flags().BoolVar(&flags.action.docker, "docker", false, wski18n.T("treat ACTION as docker image path on dockerhub"))
  actionUpdateCmd.Flags().BoolVar(&flags.action.copy, "copy", false, wski18n.T("treat ACTION as the name of an existing action"))
  actionUpdateCmd.Flags().BoolVar(&flags.action.sequence, "sequence", false, wski18n.T("treat ACTION as comma separated sequence of actions to invoke"))
  actionUpdateCmd.Flags().StringVar(&flags.action.kind, "kind", "", wski18n.T("the `KIND` of the action runtime (example: swift:3, nodejs:6)"))
  actionUpdateCmd.Flags().StringVar(&flags.action.shared, "shared", "", wski18n.T("action visibility `SCOPE`; yes = shared, no = private"))
  actionUpdateCmd.Flags().IntVarP(&flags.action.timeout, "timeout", "t", TIMEOUT_LIMIT, wski18n.T("the timeout `LIMIT` in milliseconds after which the action is terminated"))
  actionUpdateCmd.Flags().IntVarP(&flags.action.memory, "memory", "m", MEMORY_LIMIT, wski18n.T("the maximum memory `LIMIT` in MB for the action"))
  actionUpdateCmd.Flags().IntVarP(&flags.action.logsize, "logsize", "l", LOGSIZE_LIMIT, wski18n.T("the maximum log size `LIMIT` in MB for the action"))
  actionUpdateCmd.Flags().StringSliceVarP(&flags.common.annotation, "annotation", "a", []string{}, wski18n.T("annotation values in `KEY VALUE` format"))
  actionUpdateCmd.Flags().StringVarP(&flags.common.annotFile, "annotation-file", "A", "", wski18n.T("`FILE` containing annotation values in JSON format"))
  actionUpdateCmd.Flags().StringSliceVarP(&flags.common.param, "param", "p", []string{}, wski18n.T("parameter values in `KEY VALUE` format"))
  actionUpdateCmd.Flags().StringVarP(&flags.common.paramFile, "param-file", "P", "", wski18n.T("`FILE` containing parameter values in JSON format"))

  actionInvokeCmd.Flags().StringSliceVarP(&flags.common.param, "param", "p", []string{}, wski18n.T("parameter values in `KEY VALUE` format"))
  actionInvokeCmd.Flags().StringVarP(&flags.common.paramFile, "param-file", "P", "", wski18n.T("`FILE` containing parameter values in JSON format"))
  actionInvokeCmd.Flags().BoolVarP(&flags.common.blocking, "blocking", "b", false, wski18n.T("blocking invoke"))
  actionInvokeCmd.Flags().BoolVarP(&flags.action.result, "result", "r", false, wski18n.T("show only activation result if a blocking activation (unless there is a failure)"))

  actionGetCmd.Flags().BoolVarP(&flags.common.summary, "summary", "s", false, wski18n.T("summarize action details"))

  actionListCmd.Flags().IntVarP(&flags.common.skip, "skip", "s", 0, wski18n.T("exclude the first `SKIP` number of actions from the result"))
  actionListCmd.Flags().IntVarP(&flags.common.limit, "limit", "l", 30, wski18n.T("the maximum log size `LIMIT` in MB for the action"))

  actionCmd.AddCommand(
    actionCreateCmd,
    actionUpdateCmd,
    actionInvokeCmd,
    actionGetCmd,
    actionDeleteCmd,
    actionListCmd,
  )
}
