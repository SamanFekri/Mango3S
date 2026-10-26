# dmgbuild settings for the macOS disk image: a window with Mango3S.app on the
# left and an Applications shortcut on the right, so installing is a drag.
#
#   dmgbuild -s build/darwin/dmg-settings.py -D app=build/bin/Mango3S.app \
#     "Mango3S" dist/mango3s-<version>-macos-arm64.dmg
#
# Run from the repository root. `defines` is provided by dmgbuild (the -D
# values).
import os.path

app = defines.get("app", "build/bin/Mango3S.app")  # noqa: F821
appname = os.path.basename(app)

format = "UDZO"
filesystem = "HFS+"
files = [app]
symlinks = {"Applications": "/Applications"}

# 660x400 content area; the .tiff holds a 1x and a 2x (Retina) image.
background = defines.get("background", "build/darwin/dmg-background.tiff")  # noqa: F821
window_rect = ((200, 140), (660, 400))
default_view = "icon-view"
show_status_bar = False
show_tab_view = False
show_toolbar = False
show_pathbar = False
show_sidebar = False

icon_size = 112
text_size = 13
icon_locations = {
    appname: (165, 180),
    "Applications": (495, 180),
}
hide_extensions = [appname]
