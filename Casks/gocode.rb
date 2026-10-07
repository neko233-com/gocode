cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.18.0"
  sha256 arm: "c5def9243685f596236c19af667cc70959a649e0c1a5f2487854e5aa35ff0d40",
         intel: "a422961af5a6ba602c99b7f6460d3e863a5d1bfe79261fe0ed33f71dac7cf261"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
