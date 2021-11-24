// Copyright IBM Corp. 2017, 2021 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package ibm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	st "github.com/IBM-Cloud/power-go-client/clients/instance"
)

func TestAccIBMPIVPNConnectionBasic(t *testing.T) {
	connectionRes := "ibm_pi_vpn_connection.vpn"
	name := fmt.Sprintf("tf-pi-vpn-%d", acctest.RandIntRange(10, 100))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckIBMPIVPNConnectionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckIBMPIVPNConnectionConfig(name),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckIBMPIVPNConnectionExists(connectionRes),
					resource.TestCheckResourceAttr(connectionRes, "pi_vpn_connection_name", name),
					resource.TestCheckResourceAttrSet(connectionRes, "connection_id"),
					resource.TestCheckResourceAttrSet(connectionRes, "connection_status"),
					resource.TestCheckResourceAttr(connectionRes, "pi_networks.#", "1"),
					resource.TestCheckResourceAttr(connectionRes, "pi_peer_subnets.#", "1"),
				),
			},
			{
				Config: testAccCheckIBMPIVPNConnectionNetworkSubnetConfig(name),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckIBMPIVPNConnectionExists(connectionRes),
					resource.TestCheckResourceAttr(connectionRes, "pi_vpn_connection_name", name),
					resource.TestCheckResourceAttrSet(connectionRes, "connection_status"),
					resource.TestCheckResourceAttr(connectionRes, "pi_networks.#", "2"),
					resource.TestCheckResourceAttr(connectionRes, "pi_peer_subnets.#", "2"),
				),
			},
		},
	})
}
func testAccCheckIBMPIVPNConnectionDestroy(s *terraform.State) error {
	sess, err := testAccProvider.Meta().(ClientSession).IBMPISession()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "ibm_pi_vpn_connection" {
			continue
		}
		parts, err := idParts(rs.Primary.ID)
		if err != nil {
			return err
		}
		cloudInstanceID := parts[0]
		vpnConnectionID := parts[1]
		client := st.NewIBMPIVpnConnectionClient(sess, cloudInstanceID)
		_, err = client.Get(vpnConnectionID, cloudInstanceID)
		if err == nil {
			return fmt.Errorf("vpn connection still exists: %s", rs.Primary.ID)
		}
	}
	return nil
}
func testAccCheckIBMPIVPNConnectionExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}
		if rs.Primary.ID == "" {
			return errors.New("No Record ID is set")
		}

		sess, err := testAccProvider.Meta().(ClientSession).IBMPISession()
		if err != nil {
			return err
		}
		parts, err := idParts(rs.Primary.ID)
		if err != nil {
			return err
		}
		cloudInstanceID := parts[0]
		vpnConnectionID := parts[1]
		client := st.NewIBMPIVpnConnectionClient(sess, cloudInstanceID)

		_, err = client.Get(vpnConnectionID, cloudInstanceID)
		if err != nil {
			return err
		}

		return nil
	}
}

func testAccCheckIBMPIVPNConnectionConfig(name string) string {
	return fmt.Sprintf(`
	resource "ibm_pi_vpn_connection" "vpn" {
		pi_cloud_instance_id = "%[1]s"
		pi_vpn_connection_name = "%[2]s"
		pi_ike_policy_id = ibm_pi_ike_policy.ike_policy.policy_id
		pi_ipsec_policy_id = ibm_pi_ipsec_policy.ipsec_policy.policy_id
		pi_vpn_connection_mode = "policy"
		pi_networks = [ibm_pi_network.private_network1.network_id]
		pi_peer_gateway_address = "1.22.124.1"
		pi_peer_subnets = ["107.0.0.0/24"]
	}
	resource "ibm_pi_ike_policy" "ike_policy" {
		pi_cloud_instance_id = "%[1]s"
		pi_policy_name = "%[2]s"
		pi_policy_dh_group = 1
		pi_policy_encryption = "3des-cbc"
		pi_policy_key_lifetime = 180
		pi_policy_preshared_key = "sample"
		pi_policy_version = 1
	}
	resource "ibm_pi_ipsec_policy" "ipsec_policy" {
		pi_cloud_instance_id = "%[1]s"
		pi_policy_name = "%[2]s"
		pi_policy_dh_group = 1
		pi_policy_encryption = "3des-cbc"
		pi_policy_key_lifetime = 180
		pi_policy_pfs = true
		pi_policy_authentication = "hmac-md5-96"
	}
	resource "ibm_pi_network" "private_network1" {
		pi_cloud_instance_id	= "%[1]s"
		pi_network_name			= "%[2]s-net1"
		pi_network_type         = "vlan"
		pi_cidr         		= "192.35.161.0/24"
	}
	`, pi_cloud_instance_id, name)
}

func testAccCheckIBMPIVPNConnectionNetworkSubnetConfig(name string) string {
	return fmt.Sprintf(`
	resource "ibm_pi_vpn_connection" "vpn" {
		pi_cloud_instance_id = "%[1]s"
		pi_vpn_connection_name = "%[2]s"
		pi_ike_policy_id = ibm_pi_ike_policy.ike_policy.policy_id
		pi_ipsec_policy_id = ibm_pi_ipsec_policy.ipsec_policy.policy_id
		pi_vpn_connection_mode = "policy"
		pi_networks = [ ibm_pi_network.private_network1.network_id, ibm_pi_network.private_network2.network_id ]
		pi_peer_gateway_address = "1.22.124.2"
		pi_peer_subnets = ["107.0.0.0/24","199.166.0.0/24"]
	}
	resource "ibm_pi_ike_policy" "ike_policy" {
		pi_cloud_instance_id = "%[1]s"
		pi_policy_name = "%[2]s"
		pi_policy_dh_group = 1
		pi_policy_encryption = "3des-cbc"
		pi_policy_key_lifetime = 180
		pi_policy_preshared_key = "sample"
		pi_policy_version = 1
	}
	resource "ibm_pi_ipsec_policy" "ipsec_policy" {
		pi_cloud_instance_id = "%[1]s"
		pi_policy_name = "%[2]s"
		pi_policy_dh_group = 1
		pi_policy_encryption = "3des-cbc"
		pi_policy_key_lifetime = 180
		pi_policy_pfs = true
		pi_policy_authentication = "hmac-md5-96"
	}
	resource "ibm_pi_network" "private_network1" {
		pi_cloud_instance_id	= "%[1]s"
		pi_network_name			= "%[2]s-net1"
		pi_network_type         = "vlan"
		pi_cidr         		= "192.35.161.0/24"
	}
	resource "ibm_pi_network" "private_network2" {
		pi_cloud_instance_id	= "%[1]s"
		pi_network_name			= "%[2]s-net2"
		pi_network_type         = "vlan"
		pi_cidr         		= "192.35.162.0/24"
	}
	`, pi_cloud_instance_id, name)
}
