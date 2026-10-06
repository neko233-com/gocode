cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.12.0"
  sha256 arm: "67cb151aa9b164d159882a03969ae2f393a3bbaf9b631878557cb219ee6b2e22",
         intel: "fafabe43d37f689c0b4e381387541d8517d215c64afb03f5389fbb510f390ba3"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
