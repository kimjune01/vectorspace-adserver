package main

import (
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/acm"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudfront"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ebs"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// envOrConfig returns the environment variable value if set, otherwise falls back to Pulumi config.
func envOrConfig(conf *config.Config, envKey, configKey string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return conf.Get(configKey)
}

func requireEnvOrConfig(conf *config.Config, envKey, configKey string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return conf.Require(configKey)
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "")

		domain := requireEnvOrConfig(conf, "DOMAIN", "domain")
		instanceType := envOrConfig(conf, "INSTANCE_TYPE", "instanceType")
		if instanceType == "" {
			instanceType = "t3.micro"
		}
		keyName := envOrConfig(conf, "KEY_NAME", "keyName")
		adminPassword := pulumi.ToSecret(pulumi.String(requireEnvOrConfig(conf, "ADMIN_PASSWORD", "adminPassword"))).(pulumi.StringOutput)
		anthropicAPIKey := pulumi.ToSecret(pulumi.String(requireEnvOrConfig(conf, "ANTHROPIC_API_KEY", "anthropicApiKey"))).(pulumi.StringOutput)
		serverImage := envOrConfig(conf, "SERVER_IMAGE", "serverImage")
		if serverImage == "" {
			serverImage = "ghcr.io/kimjune01/vectorspace-server:latest"
		}
		hfToken := envOrConfig(conf, "HF_TOKEN", "hfToken")

		// -----------------------------------------------------------
		// Route 53 Hosted Zone
		// -----------------------------------------------------------
		zone, err := route53.NewZone(ctx, "zone", &route53.ZoneArgs{
			Name: pulumi.String(domain),
		})
		if err != nil {
			return err
		}

		// -----------------------------------------------------------
		// ACM Certificate (us-east-1 required for CloudFront)
		// -----------------------------------------------------------
		usEast1, err := aws.NewProvider(ctx, "aws-us-east-1", &aws.ProviderArgs{
			Region: pulumi.String("us-east-1"),
		})
		if err != nil {
			return err
		}

		cert, err := acm.NewCertificate(ctx, "cert", &acm.CertificateArgs{
			DomainName:       pulumi.String(domain),
			ValidationMethod: pulumi.String("DNS"),
			SubjectAlternativeNames: pulumi.StringArray{
				pulumi.Sprintf("*.%s", domain),
			},
		}, pulumi.Provider(usEast1))
		if err != nil {
			return err
		}

		certValidationRecord, err := route53.NewRecord(ctx, "cert-validation", &route53.RecordArgs{
			ZoneId:         zone.ZoneId,
			Name:           cert.DomainValidationOptions.Index(pulumi.Int(0)).ResourceRecordName().Elem(),
			Type:           cert.DomainValidationOptions.Index(pulumi.Int(0)).ResourceRecordType().Elem(),
			Ttl:            pulumi.Int(60),
			Records:        pulumi.StringArray{cert.DomainValidationOptions.Index(pulumi.Int(0)).ResourceRecordValue().Elem()},
			AllowOverwrite: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		certValidation, err := acm.NewCertificateValidation(ctx, "cert-validation-wait", &acm.CertificateValidationArgs{
			CertificateArn:        cert.Arn,
			ValidationRecordFqdns: pulumi.StringArray{certValidationRecord.Fqdn},
		}, pulumi.Provider(usEast1))
		if err != nil {
			return err
		}

		// -----------------------------------------------------------
		// EC2: Security Group, Instance, EBS, Elastic IP
		// -----------------------------------------------------------
		ami, err := ec2.LookupAmi(ctx, &ec2.LookupAmiArgs{
			MostRecent: pulumi.BoolRef(true),
			Filters: []ec2.GetAmiFilter{
				{Name: "name", Values: []string{"ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"}},
				{Name: "virtualization-type", Values: []string{"hvm"}},
			},
			Owners: []string{"099720109477"},
		})
		if err != nil {
			return err
		}

		sg, err := ec2.NewSecurityGroup(ctx, "server-sg", &ec2.SecurityGroupArgs{
			Description: pulumi.String("vectorspace server - HTTP, HTTPS, SSH, SMTP"),
			Ingress: ec2.SecurityGroupIngressArray{
				&ec2.SecurityGroupIngressArgs{Protocol: pulumi.String("tcp"), FromPort: pulumi.Int(22), ToPort: pulumi.Int(22), CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")}},
				&ec2.SecurityGroupIngressArgs{Protocol: pulumi.String("tcp"), FromPort: pulumi.Int(25), ToPort: pulumi.Int(25), CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")}},
				&ec2.SecurityGroupIngressArgs{Protocol: pulumi.String("tcp"), FromPort: pulumi.Int(80), ToPort: pulumi.Int(80), CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")}},
				&ec2.SecurityGroupIngressArgs{Protocol: pulumi.String("tcp"), FromPort: pulumi.Int(443), ToPort: pulumi.Int(443), CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")}},
			},
			Egress: ec2.SecurityGroupEgressArray{
				&ec2.SecurityGroupEgressArgs{Protocol: pulumi.String("-1"), FromPort: pulumi.Int(0), ToPort: pulumi.Int(0), CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")}},
			},
		})
		if err != nil {
			return err
		}

		userData := pulumi.All(adminPassword, anthropicAPIKey).ApplyT(func(args []interface{}) string {
			pw := args[0].(string)
			ak := args[1].(string)
			return userDataScript(domain, serverImage, pw, ak, hfToken)
		}).(pulumi.StringOutput)

		instanceArgs := &ec2.InstanceArgs{
			Ami:                 pulumi.String(ami.Id),
			InstanceType:        pulumi.String(instanceType),
			VpcSecurityGroupIds: pulumi.StringArray{sg.ID()},
			UserData:            userData,
			Tags:                pulumi.StringMap{"Name": pulumi.String("vectorspace-server")},
		}
		if keyName != "" {
			instanceArgs.KeyName = pulumi.StringPtr(keyName)
		}

		server, err := ec2.NewInstance(ctx, "server", instanceArgs)
		if err != nil {
			return err
		}

		eip, err := ec2.NewEip(ctx, "server-eip", &ec2.EipArgs{
			Instance: server.ID(),
			Domain:   pulumi.String("vpc"),
		})
		if err != nil {
			return err
		}

		vol, err := ebs.NewVolume(ctx, "data-vol", &ebs.VolumeArgs{
			AvailabilityZone: server.AvailabilityZone,
			Size:             pulumi.Int(20),
			Type:             pulumi.String("gp3"),
			Tags:             pulumi.StringMap{"Name": pulumi.String("vectorspace-data")},
		})
		if err != nil {
			return err
		}

		_, err = ec2.NewVolumeAttachment(ctx, "data-vol-att", &ec2.VolumeAttachmentArgs{
			DeviceName: pulumi.String("/dev/xvdf"),
			VolumeId:   vol.ID(),
			InstanceId: server.ID(),
		})
		if err != nil {
			return err
		}

		// -----------------------------------------------------------
		// S3 + CloudFront for landing page
		// -----------------------------------------------------------
		bucket, err := s3.NewBucket(ctx, "landing-bucket", &s3.BucketArgs{
			Bucket: pulumi.Sprintf("%s-landing", domain),
		})
		if err != nil {
			return err
		}

		// Upload all built landing files from landing/dist/
		landingDir := "../landing/dist"
		err = filepath.WalkDir(landingDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(landingDir, path)
			key := filepath.ToSlash(rel)

			ct := mime.TypeByExtension(filepath.Ext(path))
			if ct == "" {
				ct = "application/octet-stream"
			}

			_, err := s3.NewBucketObject(ctx, "landing-"+strings.ReplaceAll(key, "/", "-"), &s3.BucketObjectArgs{
				Bucket:      bucket.Bucket,
				Key:         pulumi.String(key),
				Source:      pulumi.NewFileAsset(path),
				ContentType: pulumi.String(ct),
			})
			return err
		})
		if err != nil {
			return err
		}

		oac, err := cloudfront.NewOriginAccessControl(ctx, "landing-oac", &cloudfront.OriginAccessControlArgs{
			Name:                          pulumi.String("vectorspace-landing-oac"),
			OriginAccessControlOriginType: pulumi.String("s3"),
			SigningBehavior:               pulumi.String("always"),
			SigningProtocol:               pulumi.String("sigv4"),
		})
		if err != nil {
			return err
		}

		originID := "s3-landing"
		dist, err := cloudfront.NewDistribution(ctx, "landing-cdn", &cloudfront.DistributionArgs{
			Origins: cloudfront.DistributionOriginArray{
				&cloudfront.DistributionOriginArgs{
					DomainName:            bucket.BucketRegionalDomainName,
					OriginAccessControlId: oac.ID(),
					OriginId:              pulumi.String(originID),
				},
			},
			Enabled:           pulumi.Bool(true),
			IsIpv6Enabled:     pulumi.Bool(true),
			DefaultRootObject: pulumi.String("index.html"),
			Aliases: pulumi.StringArray{
				pulumi.String(domain),
				pulumi.Sprintf("www.%s", domain),
			},
			DefaultCacheBehavior: &cloudfront.DistributionDefaultCacheBehaviorArgs{
				AllowedMethods:       pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
				CachedMethods:        pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
				TargetOriginId:       pulumi.String(originID),
				ViewerProtocolPolicy: pulumi.String("redirect-to-https"),
				Compress:             pulumi.Bool(true),
				ForwardedValues: &cloudfront.DistributionDefaultCacheBehaviorForwardedValuesArgs{
					QueryString: pulumi.Bool(false),
					Cookies:     &cloudfront.DistributionDefaultCacheBehaviorForwardedValuesCookiesArgs{Forward: pulumi.String("none")},
				},
			},
			PriceClass: pulumi.String("PriceClass_100"),
			Restrictions: &cloudfront.DistributionRestrictionsArgs{
				GeoRestriction: &cloudfront.DistributionRestrictionsGeoRestrictionArgs{RestrictionType: pulumi.String("none")},
			},
			ViewerCertificate: &cloudfront.DistributionViewerCertificateArgs{
				AcmCertificateArn: certValidation.CertificateArn,
				SslSupportMethod:  pulumi.String("sni-only"),
			},
		})
		if err != nil {
			return err
		}

		policyDoc := iam.GetPolicyDocumentOutput(ctx, iam.GetPolicyDocumentOutputArgs{
			Statements: iam.GetPolicyDocumentStatementArray{
				&iam.GetPolicyDocumentStatementArgs{
					Effect: pulumi.String("Allow"),
					Principals: iam.GetPolicyDocumentStatementPrincipalArray{
						&iam.GetPolicyDocumentStatementPrincipalArgs{
							Type:        pulumi.String("Service"),
							Identifiers: pulumi.StringArray{pulumi.String("cloudfront.amazonaws.com")},
						},
					},
					Actions:   pulumi.StringArray{pulumi.String("s3:GetObject")},
					Resources: pulumi.StringArray{pulumi.Sprintf("%s/*", bucket.Arn)},
					Conditions: iam.GetPolicyDocumentStatementConditionArray{
						&iam.GetPolicyDocumentStatementConditionArgs{
							Test:     pulumi.String("StringEquals"),
							Variable: pulumi.String("AWS:SourceArn"),
							Values:   pulumi.StringArray{dist.Arn},
						},
					},
				},
			},
		})

		_, err = s3.NewBucketPolicy(ctx, "landing-bucket-policy", &s3.BucketPolicyArgs{
			Bucket: bucket.Bucket,
			Policy: policyDoc.ApplyT(func(doc iam.GetPolicyDocumentResult) (string, error) {
				return doc.Json, nil
			}).(pulumi.StringOutput),
		})
		if err != nil {
			return err
		}

		// -----------------------------------------------------------
		// Route 53 Records
		// -----------------------------------------------------------
		_, err = route53.NewRecord(ctx, "root-record", &route53.RecordArgs{
			ZoneId: zone.ZoneId,
			Name:   pulumi.String(domain),
			Type:   pulumi.String("A"),
			Aliases: route53.RecordAliasArray{&route53.RecordAliasArgs{
				Name:                 dist.DomainName,
				ZoneId:               dist.HostedZoneId,
				EvaluateTargetHealth: pulumi.Bool(false),
			}},
		})
		if err != nil {
			return err
		}

		_, err = route53.NewRecord(ctx, "www-record", &route53.RecordArgs{
			ZoneId: zone.ZoneId,
			Name:   pulumi.Sprintf("www.%s", domain),
			Type:   pulumi.String("A"),
			Aliases: route53.RecordAliasArray{&route53.RecordAliasArgs{
				Name:                 dist.DomainName,
				ZoneId:               dist.HostedZoneId,
				EvaluateTargetHealth: pulumi.Bool(false),
			}},
		})
		if err != nil {
			return err
		}

		_, err = route53.NewRecord(ctx, "api-record", &route53.RecordArgs{
			ZoneId:  zone.ZoneId,
			Name:    pulumi.Sprintf("api.%s", domain),
			Type:    pulumi.String("A"),
			Ttl:     pulumi.Int(300),
			Records: pulumi.StringArray{eip.PublicIp},
		})
		if err != nil {
			return err
		}

		_, err = route53.NewRecord(ctx, "portal-record", &route53.RecordArgs{
			ZoneId:  zone.ZoneId,
			Name:    pulumi.Sprintf("portal.%s", domain),
			Type:    pulumi.String("A"),
			Ttl:     pulumi.Int(300),
			Records: pulumi.StringArray{eip.PublicIp},
		})
		if err != nil {
			return err
		}

		// Trust exchange: A record + MX record for attestation email delivery
		_, err = route53.NewRecord(ctx, "exchange-record", &route53.RecordArgs{
			ZoneId:  zone.ZoneId,
			Name:    pulumi.Sprintf("exchange.%s", domain),
			Type:    pulumi.String("A"),
			Ttl:     pulumi.Int(300),
			Records: pulumi.StringArray{eip.PublicIp},
		})
		if err != nil {
			return err
		}

		_, err = route53.NewRecord(ctx, "exchange-mx", &route53.RecordArgs{
			ZoneId:  zone.ZoneId,
			Name:    pulumi.Sprintf("exchange.%s", domain),
			Type:    pulumi.String("MX"),
			Ttl:     pulumi.Int(300),
			Records: pulumi.StringArray{pulumi.Sprintf("10 exchange.%s", domain)},
		})
		if err != nil {
			return err
		}

		// -----------------------------------------------------------
		// Outputs
		// -----------------------------------------------------------
		ctx.Export("serverIp", eip.PublicIp)
		ctx.Export("cloudfrontDomain", dist.DomainName)
		ctx.Export("nameservers", zone.NameServers)
		ctx.Export("apiUrl", pulumi.Sprintf("https://api.%s", domain))
		ctx.Export("portalUrl", pulumi.Sprintf("https://portal.%s", domain))
		ctx.Export("landingUrl", pulumi.Sprintf("https://%s", domain))
		ctx.Export("exchangeDomain", pulumi.Sprintf("exchange.%s", domain))
		ctx.Export("exchangeSmtp", pulumi.Sprintf("exchange.%s:25", domain))

		return nil
	})
}

func userDataScript(domain, serverImage, adminPassword, anthropicAPIKey, hfToken string) string {
	s := `#!/bin/bash
set -euo pipefail
exec > >(tee /var/log/user-data.log) 2>&1

echo "=== vectorspace.exchange bootstrap ==="

apt-get update -y
apt-get install -y ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
systemctl enable docker
systemctl start docker

if ! blkid /dev/xvdf; then
  mkfs.ext4 /dev/xvdf
fi
mkdir -p /data
mount /dev/xvdf /data
echo '/dev/xvdf /data ext4 defaults,nofail 0 2' >> /etc/fstab

mkdir -p /opt/vectorspace

cat > /opt/vectorspace/Caddyfile <<'CADDY'
api.__DOMAIN__ {
	reverse_proxy 127.0.0.1:8080
}

portal.__DOMAIN__ {
	reverse_proxy 127.0.0.1:8080
}
CADDY

cat > /opt/vectorspace/docker-compose.yml <<'COMPOSE'
services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    network_mode: host
    volumes:
      - /opt/vectorspace/Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config

  server:
    image: __SERVER_IMAGE__
    restart: unless-stopped
    network_mode: host
    volumes:
      - /data:/data
    command:
      - "-db-path=/data/vectorspace.db"
      - "-seed"
      - "-admin-password=__ADMIN_PASSWORD__"
      - "-anthropic-key=__ANTHROPIC_API_KEY__"
      - "-hf-token=__HF_TOKEN__"
      - "-smtp-addr=:25"
      - "-exchange-domain=exchange.__DOMAIN__"

volumes:
  caddy_data:
  caddy_config:
COMPOSE

cd /opt/vectorspace
docker compose pull
docker compose up -d

echo "=== bootstrap complete ==="
`
	s = strings.ReplaceAll(s, "__DOMAIN__", domain)
	s = strings.ReplaceAll(s, "__SERVER_IMAGE__", serverImage)
	s = strings.ReplaceAll(s, "__ADMIN_PASSWORD__", adminPassword)
	s = strings.ReplaceAll(s, "__ANTHROPIC_API_KEY__", anthropicAPIKey)
	s = strings.ReplaceAll(s, "__HF_TOKEN__", hfToken)
	return s
}
