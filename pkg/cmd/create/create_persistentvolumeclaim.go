/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package create

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	resource_requests "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	persistentVolumeLong = templates.LongDesc(i18n.T(`
		Create a persistentvolumeclaim with the specified name.`))

	persistentVolumeExample = templates.Examples(i18n.T(`
		# Create a persistentvolumeclaim
		kubectl create persistentvolumeclaim my-pvc --storage-request=1Gi

		# Create a persistentvolumeclaim with a resource limit
		kubectl create persistentvolumeclaim my-pvc --storage-request=500Mi --storage-limit=1Gi

		# Create a persistentvolumeclaim with an access mode
		kubectl create persistentvolumeclaim my-pvc --storage-request=500Mi --access-modes=ReadWriteOnce`))
)

// CreatePersistentVolumeClaimOptions is the command line options for 'create persistentvolumeclaim'
type CreatePersistentVolumeClaimOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	PrintObj func(obj runtime.Object) error

	// Name for resource (required)
	Name      string
	Namespace string
	// Storage request size (required)
	StorageRequest string
	// Storage limit size (optional)
	StorageLimit string
	// Storage class associated with this resource (optional)
	StorageClassName string
	// Access mode to perform operations against the resource (optional)
	AccessModes string

	EnforceNamespace    bool
	Client              corev1client.CoreV1Interface
	DryRunStrategy      cmdutil.DryRunStrategy
	ValidationDirective string
	Builder             *resource.Builder
	FieldManager        string
	CreateAnnotation    bool

	genericiooptions.IOStreams
}

// CreatePersistentVolumeClaimOptions initializes and returns new CreatePersistentVolumeClaimOptions instance
func NewCreatePersistentVolumeClaimOptions(ioStreams genericiooptions.IOStreams) *CreatePersistentVolumeClaimOptions {
	return &CreatePersistentVolumeClaimOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme.Scheme),
		IOStreams:  ioStreams,
	}
}

// NewCmdCreatePersistentVolumeClaim is a command to ease creating Jobs from CronJobs.
func NewCmdCreatePersistentVolumeClaim(f cmdutil.Factory, ioStreams genericiooptions.IOStreams) *cobra.Command {
	o := NewCreatePersistentVolumeClaimOptions(ioStreams)
	cmd := &cobra.Command{
		Use:                   "persistentvolumeclaim NAME [--storage-request=string] [--storage-request=string] [--storage-limit=string] [--access-modes=mode1,mode2] [--storage-class-name=string] [--dry-run=server|client|none]",
		DisableFlagsInUseLine: true,
		Aliases:               []string{"pvc"},
		Short:                 i18n.T("Create a persistentvolumeclaim with the specified name"),
		Long:                  persistentVolumeLong,
		Example:               persistentVolumeExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)

	cmdutil.AddApplyAnnotationFlags(cmd)
	cmdutil.AddValidateFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
	cmd.Flags().StringVar(&o.StorageRequest, "storage-request", o.StorageRequest, "Storage request capacity for the pvc")
	cmd.Flags().StringVar(&o.StorageLimit, "storage-limit", o.StorageLimit, "Storage limit capacity for the pvc")
	cmd.Flags().StringVar(&o.AccessModes, "access-modes", o.AccessModes, "Access Modes applied to pvc")
	cmd.Flags().StringVar(&o.StorageClassName, "storage-class-name", o.StorageClassName, "Storage class name that pvc will use")
	cmdutil.AddFieldManagerFlagVar(cmd, &o.FieldManager, "kubectl-create")
	return cmd
}

// Complete completes all the required options
func (o *CreatePersistentVolumeClaimOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	name, err := NameFromCommandArgs(cmd, args)
	if err != nil {
		return err
	}
	o.Name = name

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = corev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.CreateAnnotation = cmdutil.GetFlagBool(cmd, cmdutil.ApplyAnnotationsFlag)

	o.Builder = f.NewBuilder()

	o.DryRunStrategy, err = cmdutil.GetDryRunStrategy(cmd)
	if err != nil {
		return err
	}

	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	cmdutil.PrintFlagsWithDryRunStrategy(o.PrintFlags, o.DryRunStrategy)
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	o.PrintObj = func(obj runtime.Object) error {
		return printer.PrintObj(obj, o.Out)
	}

	o.ValidationDirective, err = cmdutil.GetValidationDirective(cmd)
	if err != nil {
		return err
	}

	return nil
}

// Validate makes sure provided values and valid persistentvolumeclaim options
func (o *CreatePersistentVolumeClaimOptions) Validate() error {
	if len(o.Name) == 0 {
		return fmt.Errorf("name must be specified")
	}
	if len(o.StorageRequest) == 0 && len(o.StorageRequest) == 0 {
		return fmt.Errorf("storage-request must be specified")
	}

	if len(o.AccessModes) != 0 {
		validModes := []string{"ReadOnlyMany", "ReadWriteMany", "ReadWriteOnce"}
		aml := strings.Split(o.AccessModes, ",")
		for _, am := range aml {
			if !slices.Contains(validModes, am) {
				return fmt.Errorf("provided access mode %s is invalid", am)
			}
		}
	}
	return nil
}

// Run performs the execution of 'create persistentvolumeclaim' sub command
func (o *CreatePersistentVolumeClaimOptions) Run() error {

	pv, err := o.createPersistentVolumeClaim()
	if err != nil {
		return err
	}
	err = util.CreateOrUpdateAnnotation(o.CreateAnnotation, pv, scheme.DefaultJSONEncoder())
	if err != nil {
		return err
	}
	if o.DryRunStrategy != cmdutil.DryRunClient {
		createOptions := metav1.CreateOptions{}
		if o.FieldManager != "" {
			createOptions.FieldManager = o.FieldManager
		}
		createOptions.FieldValidation = o.ValidationDirective
		if o.DryRunStrategy == cmdutil.DryRunServer {
			createOptions.DryRun = []string{metav1.DryRunAll}
		}
		pv, err = o.Client.PersistentVolumeClaims(o.Namespace).Create(context.TODO(), pv, createOptions)
		if err != nil {
			return fmt.Errorf("failed to create persistentVolumeClaim %v", err)
		}
	}

	return o.PrintObj(pv)
}

func (o *CreatePersistentVolumeClaimOptions) createPersistentVolumeClaim() (*corev1.PersistentVolumeClaim, error) {

	namespace := ""
	if o.EnforceNamespace {
		namespace = o.Namespace
	}

	pvc := &corev1.PersistentVolumeClaim{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "PersistentVolumeClaim"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{},
	}

	ps, err := o.parseResources()
	if err != nil {
		return nil, err
	}
	pvc.Spec.Resources = ps

	if len(o.AccessModes) != 0 {
		pvc.Spec.AccessModes = o.parseAccessModes()
	}
	if len(o.StorageClassName) != 0 {
		pvc.Spec.StorageClassName = &o.StorageClassName
	}
	return pvc, nil
}

func (o *CreatePersistentVolumeClaimOptions) parseAccessModes() []corev1.PersistentVolumeAccessMode {
	accessModes := []corev1.PersistentVolumeAccessMode{}
	aml := strings.Split(o.AccessModes, ",")
	for _, am := range aml {

		accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(am))
	}
	return accessModes
}

func (o *CreatePersistentVolumeClaimOptions) parseResources() (v1.VolumeResourceRequirements, error) {
	vrr := v1.VolumeResourceRequirements{}

	rr, err := resource_requests.ParseQuantity(o.StorageRequest)
	if err != nil {
		return vrr, err
	}
	vrr.Requests = corev1.ResourceList{
		corev1.ResourceStorage: rr,
	}

	if len(o.StorageLimit) != 0 {
		rl, err := resource_requests.ParseQuantity(o.StorageLimit)
		if err != nil {
			return vrr, err
		}
		if flag := rl.Cmp(rr); flag < 1 {
			return vrr, fmt.Errorf("Resource limit is the same/less than the resource request")
		}
		vrr.Limits = corev1.ResourceList{
			v1.ResourceStorage: rl,
		}
	}
	return vrr, nil
}
