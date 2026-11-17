package main

import (
	"testing"

	"github.com/go-ole/go-ole"
)

// WMI 的 uint16/uint32/uint64 属性是数值 VARIANT，ToString() 会返回空串。
// 这个回归测试钉住 variantText 必须能转出数字，否则物理盘 MediaType/BusType
// 又会变回 Unknown。
func TestVariantTextNumeric(t *testing.T) {
	v := ole.NewVariant(ole.VT_UI2, 4)
	if got := variantText(&v); got != "4" {
		t.Fatalf("VT_UI2 -> %q, want \"4\"", got)
	}
	if v.ToString() != "" {
		t.Fatal("前提变化：go-ole 现在能直接 ToString 数值了，检查是否还需要 variantText")
	}
}

func TestDiskProfileEnumNames(t *testing.T) {
	if mediaTypeName("4") != "SSD" || mediaTypeName("3") != "HDD" {
		t.Error("MediaType 映射错误")
	}
	if busTypeName("17") != "NVMe" || busTypeName("11") != "SATA" {
		t.Error("BusType 映射错误")
	}
	if healthStatusName("0") != "Healthy" || healthStatusName("1") != "Warning" || healthStatusName("2") != "Unhealthy" {
		t.Error("HealthStatus 映射错误")
	}
}
