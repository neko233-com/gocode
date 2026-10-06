cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.13.0"
  sha256 arm: "8781bbccb2ef6d4e27547b514f56d905680a32130982a903be098bd92909f651",
         intel: "bdaa22c8474e0f58293abd16435a319cce52425cc7d367126ae5a0ca9bfca251"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
