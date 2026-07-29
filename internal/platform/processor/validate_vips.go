//go:build cgo && vips

package processor

/*
#cgo pkg-config: vips
#include <stdlib.h>
#include <vips/vips.h>

static int imagesilo_validate_decode(
	const char *input,
	int load_all_pages,
	int *width,
	int *height,
	int *pages,
	int *page_height
) {
	VipsImage *image = NULL;
	if (load_all_pages) {
		image = vips_image_new_from_file(input,
			"access", VIPS_ACCESS_SEQUENTIAL,
			"n", -1,
			"fail-on", VIPS_FAIL_ON_WARNING,
			NULL);
	} else {
		image = vips_image_new_from_file(input,
			"access", VIPS_ACCESS_SEQUENTIAL,
			"fail-on", VIPS_FAIL_ON_WARNING,
			NULL);
	}
	if (image == NULL) {
		return -1;
	}

	double average = 0;
	if (vips_avg(image, &average, NULL)) {
		g_object_unref(image);
		return -1;
	}

	int detected_pages = 1;
	if (vips_image_get_typeof(image, "n-pages") != 0 &&
		vips_image_get_int(image, "n-pages", &detected_pages)) {
		g_object_unref(image);
		return -1;
	}
	int detected_page_height = vips_image_get_height(image);
	if (vips_image_get_typeof(image, "page-height") != 0 &&
		vips_image_get_int(image, "page-height", &detected_page_height)) {
		g_object_unref(image);
		return -1;
	}

	*width = vips_image_get_width(image);
	*height = vips_image_get_height(image);
	*pages = detected_pages;
	*page_height = detected_page_height;
	g_object_unref(image);
	return 0;
}
*/
import "C"

import "unsafe"

func validateDecode(path string, format Format, width, height int) (int, error) {
	if err := Startup(); err != nil {
		return 0, err
	}
	input := C.CString(path)
	defer C.free(unsafe.Pointer(input))
	loadAllPages := C.int(0)
	if format == FormatGIF {
		loadAllPages = 1
	}
	var loadedWidth, loadedHeight, pages, pageHeight C.int
	if C.imagesilo_validate_decode(
		input,
		loadAllPages,
		&loadedWidth,
		&loadedHeight,
		&pages,
		&pageHeight,
	) != 0 {
		return 0, vipsError("validate image decode")
	}
	if int(loadedWidth) != width || pages < 1 {
		return 0, ErrInvalidImage
	}
	if format == FormatGIF {
		if int(pageHeight) != height || int(loadedHeight) != height*int(pages) {
			return 0, ErrInvalidImage
		}
		return int(pages), nil
	}
	if int(loadedHeight) != height || pages != 1 {
		return 0, ErrInvalidImage
	}
	return 1, nil
}
