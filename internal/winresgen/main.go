// Command winresgen writes rsrc_windows_amd64.syso into the current
// directory: the Windows EXE icon and version information. `go build`
// links it automatically into the Windows binary; other platforms ignore
// it. Run via `go generate ./cmd/pult` after changing the icon or
// brand.Version, and commit the result. Only this generator uses the
// winres library; pult itself does not.
package main

import (
	"image"
	"log"
	"os"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"

	"pult/internal/brand"
)

func main() {
	var images []image.Image
	for _, n := range []int{16, 24, 32, 48, 64, 128, 256} {
		images = append(images, brand.Icon(n))
	}
	icon, err := winres.NewIconFromImages(images)
	if err != nil {
		log.Fatal(err)
	}

	rs := &winres.ResourceSet{}
	if err := rs.SetIcon(winres.Name("APP"), icon); err != nil {
		log.Fatal(err)
	}

	var vi version.Info
	vi.SetFileVersion(brand.Version)
	vi.SetProductVersion(brand.Version)
	for key, value := range map[string]string{
		version.ProductName:      "pult",
		version.FileDescription:  "pult - local tool launcher",
		version.InternalName:     "pult",
		version.OriginalFilename: "pult.exe",
	} {
		if err := vi.Set(version.LangDefault, key, value); err != nil {
			log.Fatal(err)
		}
	}
	rs.SetVersionInfo(vi)

	f, err := os.Create("rsrc_windows_amd64.syso")
	if err != nil {
		log.Fatal(err)
	}
	if err := rs.WriteObject(f, winres.ArchAMD64); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}
