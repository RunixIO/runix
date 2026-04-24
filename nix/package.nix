{
  lib,
  buildGoModule,
  src,
  version ? "dev",
}:
buildGoModule {
  pname = "runix";
  inherit version src;

  vendorHash = "sha256-eEhTdpZH0PExdlKylWmSKxPnUzPC8mHuoJBp+ys6TFw=";

  proxyVendor = true;

  env.CGO_ENABLED = 0;

  ldflags = [
    "-s"
    "-w"
    "-X github.com/runixio/runix/internal/version.Version=${version}"
    "-X github.com/runixio/runix/internal/version.BuildTime=1970-01-01T00:00:00Z"
  ];

  # e2e tests need a running supervisor
  checkFlags = [ "-short" ];

  meta = {
    description = "A modern process manager and application supervisor";
    homepage = "https://github.com/runixio/runix";
    license = lib.licenses.mit;
    mainProgram = "runix";
    maintainers = [ ];
    platforms = lib.platforms.linux ++ lib.platforms.darwin;
  };
}
