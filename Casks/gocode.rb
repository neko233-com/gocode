cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.19.0"
  sha256 arm: "0f559afcad7e9d87caffc13f7c76dcff3b25b1b1904e1952e4163d7707fddb40",
         intel: "d6b680c3792ac788a3a0624a56868c696f9751c5fd8cb373dd8dadb878815d88"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
