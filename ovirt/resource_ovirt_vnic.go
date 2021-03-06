// Copyright (C) 2018 Joey Ma <majunjiev@gmail.com>
// All rights reserved.
//
// This software may be modified and distributed under the terms
// of the BSD-2 license.  See the LICENSE file for details.

package ovirt

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	ovirtsdk4 "gopkg.in/imjoey/go-ovirt.v4"
)

func resourceOvirtVnic() *schema.Resource {
	return &schema.Resource{
		Create: resourceOvirtVnicCreate,
		Read:   resourceOvirtVnicRead,
		Delete: resourceOvirtVnicDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"vm_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"vnic_profile_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceOvirtVnicCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*ovirtsdk4.Connection)
	vmService := conn.SystemService().
		VmsService().
		VmService(d.Get("vm_id").(string))

	addVnicResp, err := vmService.NicsService().
		Add().
		Nic(
			ovirtsdk4.NewNicBuilder().
				Name(d.Get("name").(string)).
				VnicProfile(
					ovirtsdk4.NewVnicProfileBuilder().
						Id(d.Get("vnic_profile_id").(string)).
						MustBuild()).
				MustBuild()).
		Send()
	if err != nil {
		return err
	}
	vnic, ok := addVnicResp.Nic()
	if !ok {
		return fmt.Errorf("failed to add nic: response not contains the nic")
	}

	// The vnic could not be fetched via the vnic ID alone.
	d.SetId(d.Get("vm_id").(string) + ":" + vnic.MustId())
	return resourceOvirtVnicRead(d, meta)
}

func resourceOvirtVnicRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*ovirtsdk4.Connection)
	vmID, vnicID, err := getVMIDAndNicID(d.Id())
	if err != nil {
		return err
	}
	d.Set("vm_id", vmID)

	getVnicResp, err := conn.SystemService().
		VmsService().
		VmService(vmID).
		NicsService().
		NicService(vnicID).
		Get().
		Send()
	if err != nil {
		if _, ok := err.(*ovirtsdk4.NotFoundError); ok {
			d.SetId("")
			return nil
		}
		return err
	}

	d.Set("name", getVnicResp.MustNic().MustName())
	d.Set("vnic_profile_id", getVnicResp.MustNic().MustVnicProfile().MustId())

	return nil
}

func resourceOvirtVnicDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*ovirtsdk4.Connection)
	vmID, vnicID, err := getVMIDAndNicID(d.Id())
	if err != nil {
		return err
	}

	nicService := conn.SystemService().
		VmsService().
		VmService(vmID).
		NicsService().
		NicService(vnicID)

	log.Printf("[DEBUG] Deactivate nic (%s) before remove", vnicID)
	_, err = nicService.Deactivate().Send()
	if err != nil {
		if _, ok := err.(*ovirtsdk4.NotFoundError); ok {
			return nil
		}
		return err
	}

	log.Printf("[DEBUG] Now to remove nic (%s) ", vnicID)
	_, err = nicService.Remove().Send()
	if err != nil {
		if _, ok := err.(*ovirtsdk4.NotFoundError); ok {
			return nil
		}
		return err
	}
	return nil
}

func getVMIDAndNicID(id string) (string, string, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid resource id")
	}
	return parts[0], parts[1], nil
}
