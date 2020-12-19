source = [
  "./dist/kit-macos_darwin_amd64/kit"]
bundle_id = "org.makeos.kit"

apple_id {
  username = "kennedyidialu@gmail.com"
  password = "@env:AC_PASSWORD"
}

sign {
  application_identity = "Developer ID Application: Kennedy Idialu (3J2BDPDMHD)"
}