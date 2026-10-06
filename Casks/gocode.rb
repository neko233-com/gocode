cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.9.0"
  sha256 arm: "31d0d4f9407e57094e7a8cae296d6e13c7bc345f290adf53e4625ee8d84f7fdd",
         intel: "9391399cfd130065e418a675f1d6bb7132dcecfd0aacd3e2dd8becbd9074dc0b"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
