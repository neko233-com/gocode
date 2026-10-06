cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.7.0"
  sha256 arm: "cc99b64840c1ec2603de211cfd446131690c80df1bdab33872f4fc3d467ba928",
         intel: "b1d3d71b9722afe990ca65b6457d4993e59b963cc56644c27e6630af91cc67a6"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
