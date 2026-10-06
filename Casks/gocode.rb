cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.10.0"
  sha256 arm: "257778f9f5372ea9f8f8970965146b3ac9b6e61533b424b1b7abfa3d957c2bba",
         intel: "ceda48577b421c1f6e802cd0880da83d4f158e62e30550215d0cb90aad67c024"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
