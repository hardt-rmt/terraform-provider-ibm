// Copyright IBM Corp. 2017, 2021 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package vpc

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/validate"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	isFloatingIPAddress       = "address"
	isFloatingIPCRN           = "crn"
	isFloatingIPName          = "name"
	isFloatingIPStatus        = "status"
	isFloatingIPZone          = "zone"
	isFloatingIPTarget        = "target"
	isFloatingIPResourceGroup = "resource_group"
	isFloatingIPTags          = "tags"

	isFloatingIPPending   = "pending"
	isFloatingIPAvailable = "available"
	isFloatingIPDeleting  = "deleting"
	isFloatingIPDeleted   = "done"
)

func ResourceIBMISFloatingIP() *schema.Resource {
	return &schema.Resource{
		Create:   resourceIBMISFloatingIPCreate,
		Read:     resourceIBMISFloatingIPRead,
		Update:   resourceIBMISFloatingIPUpdate,
		Delete:   resourceIBMISFloatingIPDelete,
		Exists:   resourceIBMISFloatingIPExists,
		Importer: &schema.ResourceImporter{},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		CustomizeDiff: customdiff.All(
			customdiff.Sequence(
				func(_ context.Context, diff *schema.ResourceDiff, v interface{}) error {
					return flex.ResourceTagsCustomizeDiff(diff)
				},
			),
			customdiff.Sequence(
				func(_ context.Context, diff *schema.ResourceDiff, v interface{}) error {

					if diff.HasChange(isFloatingIPTarget) {
						if !diff.NewValueKnown(isFloatingIPTarget) {
							diff.ForceNew(isFloatingIPTarget)
							return nil
						}
						old, new := diff.GetChange(isFloatingIPTarget)
						if old != "" || new != "" {
							sess, err := vpcClient(v)
							if err != nil {
								return err
							}
							if checkIfZoneChanged(old.(string), new.(string), diff.Get(isFloatingIPZone).(string), sess) {
								diff.ForceNew(isFloatingIPTarget)
							}
						}
					}
					return nil
				},
			),
		),

		Schema: map[string]*schema.Schema{
			isFloatingIPAddress: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Floating IP address",
			},

			isFloatingIPName: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     false,
				ValidateFunc: validate.InvokeValidator("ibm_is_floating_ip", isFloatingIPName),
				Description:  "Name of the floating IP",
			},

			isFloatingIPStatus: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Floating IP status",
			},

			isFloatingIPZone: {
				Type:          schema.TypeString,
				ForceNew:      true,
				Optional:      true,
				Computed:      true,
				ConflictsWith: []string{isFloatingIPTarget},
				Description:   "Zone name",
			},

			isFloatingIPTarget: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ConflictsWith: []string{isFloatingIPZone},
				Description:   "Target info",
			},

			isFloatingIPResourceGroup: {
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Computed:    true,
				Description: "Resource group info",
			},

			isFloatingIPTags: {
				Type:        schema.TypeSet,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString, ValidateFunc: validate.InvokeValidator("ibm_is_floating_ip", "tag")},
				Set:         flex.ResourceIBMVPCHash,
				Description: "Floating IP tags",
			},

			flex.ResourceControllerURL: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The URL of the IBM Cloud dashboard that can be used to explore and view details about this instance",
			},

			flex.ResourceName: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The name of the resource",
			},

			flex.ResourceCRN: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The crn of the resource",
			},

			isFloatingIPCRN: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The crn of the resource",
			},

			flex.ResourceStatus: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The status of the resource",
			},

			flex.ResourceGroupName: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The resource group name in which resource is provisioned",
			},
		},
	}
}

func vpcClient(meta interface{}) (*vpcv1.VpcV1, error) {
	sess, err := meta.(conns.ClientSession).VpcV1API()
	return sess, err
}

func ResourceIBMISFloatingIPValidator() *validate.ResourceValidator {

	validateSchema := make([]validate.ValidateSchema, 0)
	validateSchema = append(validateSchema,
		validate.ValidateSchema{
			Identifier:                 isFloatingIPName,
			ValidateFunctionIdentifier: validate.ValidateRegexpLen,
			Type:                       validate.TypeString,
			Required:                   true,
			Regexp:                     `^([a-z]|[a-z][-a-z0-9]*[a-z0-9])$`,
			MinValueLength:             1,
			MaxValueLength:             63})
	validateSchema = append(validateSchema,
		validate.ValidateSchema{
			Identifier:                 "tag",
			ValidateFunctionIdentifier: validate.ValidateRegexpLen,
			Type:                       validate.TypeString,
			Optional:                   true,
			Regexp:                     `^[A-Za-z0-9:_ .-]+$`,
			MinValueLength:             1,
			MaxValueLength:             128})

	ibmISFloatingIPResourceValidator := validate.ResourceValidator{ResourceName: "ibm_is_floating_ip", Schema: validateSchema}
	return &ibmISFloatingIPResourceValidator
}

func resourceIBMISFloatingIPCreate(d *schema.ResourceData, meta interface{}) error {

	name := d.Get(isFloatingIPName).(string)
	err := fipCreate(d, meta, name)
	if err != nil {
		return err
	}

	return resourceIBMISFloatingIPRead(d, meta)
}

func fipCreate(d *schema.ResourceData, meta interface{}, name string) error {
	sess, err := vpcClient(meta)
	if err != nil {
		return err
	}

	floatingIPPrototype := &vpcv1.FloatingIPPrototype{
		Name: &name,
	}
	zone, target := "", ""
	if zn, ok := d.GetOk(isFloatingIPZone); ok {
		zone = zn.(string)
		floatingIPPrototype.Zone = &vpcv1.ZoneIdentity{
			Name: &zone,
		}
	}

	if tgt, ok := d.GetOk(isFloatingIPTarget); ok {
		target = tgt.(string)
		floatingIPPrototype.Target = &vpcv1.FloatingIPByTargetNetworkInterfaceIdentity{
			ID: &target,
		}
	}

	if zone == "" && target == "" {
		return fmt.Errorf("%s or %s need to be provided", isFloatingIPZone, isFloatingIPTarget)
	}

	if rgrp, ok := d.GetOk(isFloatingIPResourceGroup); ok {
		rg := rgrp.(string)
		floatingIPPrototype.ResourceGroup = &vpcv1.ResourceGroupIdentity{
			ID: &rg,
		}
	}

	createFloatingIPOptions := &vpcv1.CreateFloatingIPOptions{
		FloatingIPPrototype: floatingIPPrototype,
	}

	floatingip, response, err := sess.CreateFloatingIP(createFloatingIPOptions)
	if err != nil {
		return fmt.Errorf("[DEBUG] Floating IP err %s\n%s", err, response)
	}
	d.SetId(*floatingip.ID)
	log.Printf("[INFO] Floating IP : %s[%s]", *floatingip.ID, *floatingip.Address)
	_, err = isWaitForInstanceFloatingIP(sess, d.Id(), d)
	if err != nil {
		return err
	}
	v := os.Getenv("IC_ENV_TAGS")
	if _, ok := d.GetOk(isFloatingIPTags); ok || v != "" {
		oldList, newList := d.GetChange(isFloatingIPTags)
		err = flex.UpdateTagsUsingCRN(oldList, newList, meta, *floatingip.CRN)
		if err != nil {
			log.Printf(
				"Error on create of vpc Floating IP (%s) tags: %s", d.Id(), err)
		}
	}
	return nil
}

func resourceIBMISFloatingIPRead(d *schema.ResourceData, meta interface{}) error {
	id := d.Id()
	err := fipGet(d, meta, id)
	if err != nil {
		return err
	}

	return nil
}

func fipGet(d *schema.ResourceData, meta interface{}, id string) error {
	sess, err := vpcClient(meta)
	if err != nil {
		return err
	}
	getFloatingIPOptions := &vpcv1.GetFloatingIPOptions{
		ID: &id,
	}
	floatingip, response, err := sess.GetFloatingIP(getFloatingIPOptions)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("[ERROR] Error Getting Floating IP (%s): %s\n%s", id, err, response)

	}
	d.Set(isFloatingIPName, *floatingip.Name)
	d.Set(isFloatingIPAddress, *floatingip.Address)
	d.Set(isFloatingIPStatus, *floatingip.Status)
	d.Set(isFloatingIPZone, *floatingip.Zone.Name)
	target, ok := floatingip.Target.(*vpcv1.FloatingIPTarget)
	if ok {
		d.Set(isFloatingIPTarget, target.ID)
	} else {
		d.Set(isFloatingIPTarget, "")
	}
	tags, err := flex.GetTagsUsingCRN(meta, *floatingip.CRN)
	if err != nil {
		log.Printf(
			"Error on get of vpc Floating IP (%s) tags: %s", d.Id(), err)
	}
	d.Set(isFloatingIPTags, tags)
	controller, err := flex.GetBaseController(meta)
	if err != nil {
		return err
	}
	d.Set(flex.ResourceControllerURL, controller+"/vpc-ext/network/floatingIPs")
	d.Set(flex.ResourceName, *floatingip.Name)
	d.Set(isFloatingIPCRN, *floatingip.CRN)
	d.Set(flex.ResourceCRN, *floatingip.CRN)
	d.Set(flex.ResourceStatus, *floatingip.Status)
	if floatingip.ResourceGroup != nil {
		d.Set(flex.ResourceGroupName, floatingip.ResourceGroup.Name)
		d.Set(isFloatingIPResourceGroup, floatingip.ResourceGroup.ID)
	}
	return nil
}

func resourceIBMISFloatingIPUpdate(d *schema.ResourceData, meta interface{}) error {
	id := d.Id()
	err := fipUpdate(d, meta, id)
	if err != nil {
		return err
	}
	return resourceIBMISFloatingIPRead(d, meta)
}

func fipUpdate(d *schema.ResourceData, meta interface{}, id string) error {
	sess, err := vpcClient(meta)
	if err != nil {
		return err
	}
	if d.HasChange(isFloatingIPTags) {
		options := &vpcv1.GetFloatingIPOptions{
			ID: &id,
		}
		fip, response, err := sess.GetFloatingIP(options)
		if err != nil {
			return fmt.Errorf("[ERROR] Error getting Floating IP: %s\n%s", err, response)
		}
		oldList, newList := d.GetChange(isFloatingIPTags)
		err = flex.UpdateTagsUsingCRN(oldList, newList, meta, *fip.CRN)
		if err != nil {
			log.Printf(
				"Error on update of vpc Floating IP (%s) tags: %s", id, err)
		}
	}
	hasChanged := false
	options := &vpcv1.UpdateFloatingIPOptions{
		ID: &id,
	}
	floatingIPPatchModel := &vpcv1.FloatingIPPatch{}
	if d.HasChange(isFloatingIPName) {
		name := d.Get(isFloatingIPName).(string)
		floatingIPPatchModel.Name = &name
		hasChanged = true
		floatingIPPatch, err := floatingIPPatchModel.AsPatch()
		if err != nil {
			return fmt.Errorf("[ERROR] Error calling asPatch for FloatingIPPatch: %s", err)
		}
		options.FloatingIPPatch = floatingIPPatch
	}

	if d.HasChange(isFloatingIPTarget) {
		target := d.Get(isFloatingIPTarget).(string)
		floatingIPPatchModel.Target = &vpcv1.FloatingIPPatchTargetNetworkInterfaceIdentity{
			ID: &target,
		}
		hasChanged = true
		floatingIPPatch, err := floatingIPPatchModel.AsPatch()
		if err != nil {
			return fmt.Errorf("[ERROR] Error calling asPatch for floatingIPPatch: %s", err)
		}
		options.FloatingIPPatch = floatingIPPatch
	}
	if hasChanged {
		_, response, err := sess.UpdateFloatingIP(options)
		if err != nil {
			return fmt.Errorf("[ERROR] Error updating vpc Floating IP: %s\n%s", err, response)
		}
	}
	return nil
}

func resourceIBMISFloatingIPDelete(d *schema.ResourceData, meta interface{}) error {
	id := d.Id()
	err := fipDelete(d, meta, id)
	if err != nil {
		return err
	}
	return nil
}

func fipDelete(d *schema.ResourceData, meta interface{}, id string) error {
	sess, err := vpcClient(meta)
	if err != nil {
		return err
	}
	getFloatingIpOptions := &vpcv1.GetFloatingIPOptions{
		ID: &id,
	}
	_, response, err := sess.GetFloatingIP(getFloatingIpOptions)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			return nil
		}

		return fmt.Errorf("[ERROR] Error Getting Floating IP (%s): %s\n%s", id, err, response)
	}

	options := &vpcv1.DeleteFloatingIPOptions{
		ID: &id,
	}
	response, err = sess.DeleteFloatingIP(options)
	if err != nil {
		return fmt.Errorf("[ERROR] Error Deleting Floating IP : %s\n%s", err, response)
	}
	_, err = isWaitForFloatingIPDeleted(sess, id, d.Timeout(schema.TimeoutDelete))
	if err != nil {
		return err
	}
	d.SetId("")
	return nil
}

func resourceIBMISFloatingIPExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	id := d.Id()
	exists, err := fipExists(d, meta, id)
	return exists, err
}

func fipExists(d *schema.ResourceData, meta interface{}, id string) (bool, error) {
	sess, err := vpcClient(meta)
	if err != nil {
		return false, err
	}
	getFloatingIpOptions := &vpcv1.GetFloatingIPOptions{
		ID: &id,
	}
	_, response, err := sess.GetFloatingIP(getFloatingIpOptions)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			return false, nil
		}
		return false, fmt.Errorf("[ERROR] Error getting floating IP: %s\n%s", err, response)
	}
	return true, nil
}

func isWaitForFloatingIPDeleted(fip *vpcv1.VpcV1, id string, timeout time.Duration) (interface{}, error) {
	log.Printf("Waiting for FloatingIP (%s) to be deleted.", id)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{isFloatingIPPending, isFloatingIPDeleting},
		Target:     []string{"", isFloatingIPDeleted},
		Refresh:    isFloatingIPDeleteRefreshFunc(fip, id),
		Timeout:    timeout,
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}

	return stateConf.WaitForState()
}

func isFloatingIPDeleteRefreshFunc(fip *vpcv1.VpcV1, id string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		log.Printf("[DEBUG] floating ip delete function here")
		getfipoptions := &vpcv1.GetFloatingIPOptions{
			ID: &id,
		}
		FloatingIP, response, err := fip.GetFloatingIP(getfipoptions)
		if err != nil {
			if response != nil && response.StatusCode == 404 {
				return FloatingIP, isFloatingIPDeleted, nil
			}
			return FloatingIP, "", fmt.Errorf("[ERROR] Error Getting Floating IP: %s\n%s", err, response)
		}
		return FloatingIP, isFloatingIPDeleting, err
	}
}

func isWaitForInstanceFloatingIP(floatingipC *vpcv1.VpcV1, id string, d *schema.ResourceData) (interface{}, error) {
	log.Printf("Waiting for floating IP (%s) to be available.", id)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{isFloatingIPPending},
		Target:     []string{isFloatingIPAvailable, ""},
		Refresh:    isInstanceFloatingIPRefreshFunc(floatingipC, id),
		Timeout:    d.Timeout(schema.TimeoutCreate),
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}

	return stateConf.WaitForState()
}

func isInstanceFloatingIPRefreshFunc(floatingipC *vpcv1.VpcV1, id string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		getfipoptions := &vpcv1.GetFloatingIPOptions{
			ID: &id,
		}
		instance, response, err := floatingipC.GetFloatingIP(getfipoptions)
		if err != nil {
			return nil, "", fmt.Errorf("[ERROR] Error Getting Floating IP for the instance: %s\n%s", err, response)
		}

		if *instance.Status == "available" {
			return instance, isFloatingIPAvailable, nil
		}

		return instance, isFloatingIPPending, nil
	}
}

func checkIfZoneChanged(oldNic, newNic, currentZone string, floatingipC *vpcv1.VpcV1) bool {
	var oldZone, newZone string
	listInstancesOptions := &vpcv1.ListInstancesOptions{}
	start := ""
	allrecs := []vpcv1.Instance{}
	for {

		if start != "" {
			listInstancesOptions.Start = &start
		}

		instances, _, err := floatingipC.ListInstances(listInstancesOptions)
		if err != nil {
			return false
		}
		start = flex.GetNext(instances.Next)
		allrecs = append(allrecs, instances.Instances...)
		if start == "" {
			break
		}
	}
	for _, instance := range allrecs {
		for _, nic := range instance.NetworkInterfaces {
			if oldNic == *nic.ID {
				oldZone = *instance.Zone.Name
			}
			if newNic == *nic.ID {
				newZone = *instance.Zone.Name
			}
		}
	}
	if newZone != oldZone {
		if oldZone == "" && newZone == currentZone {
			return false
		}
		return true
	}
	return false
}
