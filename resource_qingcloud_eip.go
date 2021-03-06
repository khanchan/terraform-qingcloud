package qingcloud

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/magicshui/qingcloud-go/eip"
)

func resourceQingcloudEip() *schema.Resource {
	return &schema.Resource{
		Create: resourceQingcloudEipCreate,
		Read:   resourceQingcloudEipRead,
		Update: resourceQingcloudEipUpdate,
		Delete: resourceQingcloudEipDelete,
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "公网 IP 的名称",
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"bandwidth": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "公网IP带宽上限，单位为Mbps",
			},
			"billing_mode": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "traffic",
				Description:  "公网IP计费模式：bandwidth 按带宽计费，traffic 按流量计费，默认是 bandwidth",
				ValidateFunc: withinArrayString("traffic", "bandwidth"),
			},
			"need_icp": &schema.Schema{
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				Description:  "是否需要备案，1为需要，0为不需要，默认是0",
				ValidateFunc: withinArrayInt(0, 1),
			},
			// -------------------------------------------
			// ----------    如下是自动计算的     -----------
			"addr": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"status": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"transition_status": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			// 目前正在使用这个 IP 的资源
			"resource": &schema.Schema{
				Type:         schema.TypeMap,
				Computed:     true,
				ComputedWhen: []string{"id"},
			},
			"id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func resourceQingcloudEipCreate(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).eip
	params := eip.AllocateEipsRequest{}
	params.Bandwidth.Set(d.Get("bandwidth").(int))
	params.BillingMode.Set(d.Get("billing_mode").(string))
	params.EipName.Set(d.Get("name").(string))
	params.NeedIcp.Set(d.Get("need_icp").(int))
	resp, err := clt.AllocateEips(params)
	if err != nil {
		return fmt.Errorf("Error create eip ", err)
	}
	d.SetId(resp.Eips[0])

	// 设置描述信息
	if err := modifyEipAttributes(d, meta, true); err != nil {
		return err
	}

	// 配置一下
	return resourceQingcloudEipRead(d, meta)
}

func resourceQingcloudEipRead(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).eip
	_, err := EipTransitionStateRefresh(clt, d.Id())
	if err != nil {
		return fmt.Errorf(
			"Error waiting for the transition %s", err)
	}

	// 设置请求参数
	params := eip.DescribeEipsRequest{}
	params.EipsN.Add(d.Id())
	params.Verbose.Set(1)

	resp, err := clt.DescribeEips(params)
	if err != nil {
		return fmt.Errorf("Error retrieving eips: %s", err)
	}
	if len(resp.EipSet) == 0 {
		return fmt.Errorf("Not found", nil)
	}

	sg := resp.EipSet[0]

	d.Set("name", sg.EipName)
	d.Set("billing_mode", sg.BillingMode)
	d.Set("bandwidth", sg.Bandwidth)
	d.Set("need_icp", sg.NeedIcp)
	d.Set("description", sg.Description)
	// 如下状态是稍等来获取的
	d.Set("addr", sg.EipAddr)
	d.Set("status", sg.Status)
	d.Set("transition_status", sg.TransitionStatus)
	if err := d.Set("resource", getEipSourceMap(sg)); err != nil {
		return fmt.Errorf("Error set eip resource %v", err)
	}
	return nil
}

func resourceQingcloudEipDelete(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).eip

	params := eip.ReleaseEipsRequest{}
	params.EipsN.Add(d.Id())
	_, err := clt.ReleaseEips(params)
	if err != nil {
		return fmt.Errorf("Error delete eip %s", err)
	}
	d.SetId("")
	return nil
}

func resourceQingcloudEipUpdate(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).eip

	if !d.HasChange("name") && !d.HasChange("description") && !d.HasChange("bandwidth") && !d.HasChange("billing_mode") {
		return nil
	}

	if d.HasChange("bandwidth") {
		params := eip.ChangeEipsBandwidthRequest{}
		params.EipsN.Add(d.Id())
		params.Bandwidth.Set(d.Get("bandwidth").(int))
		_, err := clt.ChangeEipsBandwidth(params)
		if err != nil {
			return err
		}
	}

	if d.HasChange("billing_mode") {
		params := eip.ChangeEipsBillingModeRequest{}
		params.EipsN.Add(d.Id())
		params.BillingMode.Set(d.Get("billing_mode").(string))
		_, err := clt.ChangeEipsBillingMode(params)
		if err != nil {
			return err
		}
	}

	if err := modifyEipAttributes(d, meta, false); err != nil {
		return err
	}

	return resourceQingcloudEipRead(d, meta)
}
