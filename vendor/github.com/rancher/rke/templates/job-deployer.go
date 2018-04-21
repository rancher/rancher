package templates

const JobDeployerTemplate = `
{{- $addonName := .AddonName }}
{{- $nodeName := .NodeName }}
{{- $image := .Image }}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{$addonName}}-deploy-job
spec:
  backoffLimit: 10
  template:
    metadata:
       name: pi
    spec:
        hostNetwork: true
        serviceAccountName: rke-job-deployer
        nodeName: {{$nodeName}}
        containers:
          - name: {{$addonName}}-pod
            image: {{$image}}
            command: [ "kubectl", "apply", "-f" , "/etc/config/{{$addonName}}.yaml"]
            volumeMounts:
            - name: config-volume
              mountPath: /etc/config
        volumes:
          - name: config-volume
            configMap:
              # Provide the name of the ConfigMap containing the files you want
              # to add to the container
              name: {{$addonName}}
              items:
                - key: {{$addonName}}
                  path: {{$addonName}}.yaml
        restartPolicy: Never`
