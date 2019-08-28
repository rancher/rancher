package templates

const AddonJobTemplate = `
{{- $addonName := .AddonName }}
{{- $nodeName := .NodeName }}
{{- $image := .Image }}
apiVersion: batch/v1
kind: Job
metadata:
{{- if eq .DeleteJob "true" }}
  name: {{$addonName}}-delete-job
{{- else }}
  name: {{$addonName}}-deploy-job
{{- end }}
  namespace: kube-system
spec:
  backoffLimit: 10
  template:
    metadata:
       name: rke-deploy
    spec:
        affinity:
          nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
              nodeSelectorTerms:
                - matchExpressions:
                  - key: beta.kubernetes.io/os
                    operator: NotIn
                    values:
                      - windows
        tolerations:
        - operator: Exists
        hostNetwork: true
        serviceAccountName: rke-job-deployer
        nodeName: {{$nodeName}}
        containers:
          {{- if eq .DeleteJob "true" }}
          - name: {{$addonName}}-delete-pod
          {{- else }}
          - name: {{$addonName}}-pod
          {{- end }}
            image: {{$image}}
            {{- if eq .DeleteJob "true" }}
            command: ["/bin/sh"]
            args: ["-c" ,"kubectl get --ignore-not-found=true -f /etc/config/{{$addonName}}.yaml -o name | xargs kubectl delete --ignore-not-found=true"]
            {{- else }}
            command: [ "kubectl", "apply", "-f" , "/etc/config/{{$addonName}}.yaml"]
            {{- end }}
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
