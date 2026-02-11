package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"time"

	"github.com/dimkr/tootik/cluster"
	"github.com/dimkr/tootik/outbox"
)

func generateCert(cn string) ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotAfter:    time.Now().AddDate(0, 0, 1),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		return nil, nil, err
	}

	var certPEM bytes.Buffer
	if err := pem.Encode(&certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return nil, nil, err
	}

	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	var keyPEM bytes.Buffer
	if err := pem.Encode(&keyPEM, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return nil, nil, err
	}

	return certPEM.Bytes(), keyPEM.Bytes(), nil
}

func generateKeypair(cn string) tls.Certificate {
	cert, key, err := generateCert(cn)
	if err != nil {
		panic(err)
	}

	keypair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		panic(err)
	}

	return keypair
}

func generateKeypairs() map[string]tls.Certificate {
	cns := []string{"alice", "bob", "carol", "dave", "eve", "frank", "grace", "heidi", "ivan", "judy"}
	pairs := make(map[string]tls.Certificate, len(cns))

	for _, cn := range cns {
		pairs[cn] = generateKeypair(cn)
	}

	return pairs
}

func seed(t cluster.T, keyPairs map[string]tls.Certificate) cluster.Cluster {
	cl := cluster.NewCluster(t, "pizza.example", "sushi.example", "pasta.example")

	alice := cl["pizza.example"].Register(keyPairs["alice"]).OK()
	bob := cl["pizza.example"].Register(keyPairs["bob"]).OK()
	carol := cl["sushi.example"].Register(keyPairs["carol"]).OK()
	dave := cl["sushi.example"].Register(keyPairs["dave"]).OK()
	eve := cl["pasta.example"].Register(keyPairs["eve"]).OK()
	frank := cl["pizza.example"].Register(keyPairs["frank"]).OK()
	grace := cl["sushi.example"].Register(keyPairs["grace"]).OK()
	heidi := cl["pasta.example"].Register(keyPairs["heidi"]).OK()
	ivan := cl["pizza.example"].Register(keyPairs["ivan"]).OK()
	judy := cl["sushi.example"].Register(keyPairs["judy"]).OK()

	alice.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Hey there! I'm Alice. I'm a total tech geek and I'm always on the hunt for the perfect pizza slice. Let's talk tech and toppings!").OK()
	bob.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "I've been called a tough critic, but I just know what I like. Looking for the best pizza in town – any recommendations?").OK()
	carol.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Exploring the world of sushi, one roll at a time. I can be a bit skeptical, but I'm always open to being pleasantly surprised!").OK()
	dave.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Just your local explorer looking for the next great meal. I love discovering hidden gems and meeting new people along the way.").OK()
	eve.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Hi! I'm Eve. Nothing makes me happier than a big plate of pasta and good company. Let's be friends!").OK()
	frank.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Team white sauce all the way! 🍕 If you think tomato sauce is the only option, let's have a friendly debate.").OK()
	grace.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Sushi lover by day, pizza sauce expert by night. I'm all about finding that perfect balance of flavors.").OK()
	heidi.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Hi everyone! I'm Heidi. I live for pasta and love sharing my favorite food finds. Can't wait to see what you're all eating!").OK()
	ivan.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Always trying to stay ahead of the pizza curve. If there's a new trend or a weird topping, I'm probably trying it right now!").OK()
	judy.Follow("⚙️ Settings").Follow("📜 Bio").FollowInput("Set", "Sushi fan and bowling enthusiast. 🍣🎳 Whether it's a new roll or a night at the lanes, I'm always up for an adventure!").OK()
	cl.Settle(t)

	alice.FollowInput("🔭 View profile", "carol@sushi.example").Follow("⚡ Follow carol").OK()
	alice.FollowInput("🔭 View profile", "dave@sushi.example").Follow("⚡ Follow dave").OK()
	alice.FollowInput("🔭 View profile", "heidi@pasta.example").Follow("⚡ Follow heidi").OK()
	bob.FollowInput("🔭 View profile", "alice@pizza.example").Follow("⚡ Follow alice").OK()
	bob.FollowInput("🔭 View profile", "eve@pasta.example").Follow("⚡ Follow eve").OK()
	carol.FollowInput("🔭 View profile", "eve@pasta.example").Follow("⚡ Follow eve").OK()
	carol.FollowInput("🔭 View profile", "frank@pizza.example").Follow("⚡ Follow frank").OK()
	dave.FollowInput("🔭 View profile", "alice@pizza.example").Follow("⚡ Follow alice").OK()
	dave.FollowInput("🔭 View profile", "bob@pizza.example").Follow("⚡ Follow bob").OK()
	eve.FollowInput("🔭 View profile", "dave@sushi.example").Follow("⚡ Follow dave").OK()
	eve.FollowInput("🔭 View profile", "alice@pizza.example").Follow("⚡ Follow alice").OK()
	frank.FollowInput("🔭 View profile", "grace@sushi.example").Follow("⚡ Follow grace").OK()
	alice.FollowInput("🔭 View profile", "judy@sushi.example").Follow("⚡ Follow judy").OK()
	grace.FollowInput("🔭 View profile", "judy@sushi.example").Follow("⚡ Follow judy").OK()
	carol.FollowInput("🔭 View profile", "ivan@pizza.example").Follow("⚡ Follow ivan").OK()
	heidi.FollowInput("🔭 View profile", "ivan@pizza.example").Follow("⚡ Follow ivan").OK()
	heidi.FollowInput("🔭 View profile", "judy@sushi.example").Follow("⚡ Follow judy").OK()
	ivan.FollowInput("🔭 View profile", "bob@pizza.example").Follow("⚡ Follow bob").OK()
	judy.FollowInput("🔭 View profile", "carol@sushi.example").Follow("⚡ Follow carol").OK()
	cl.Settle(t)

	carolInitialPost := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "Finally tried that new pizza place everyone's been talking about. Honestly? Overrated.").
		OK()
	cl.Settle(t)

	alice.
		GotoInput(carolInitialPost.Links["💬 Reply"], "@carol No way! I loved it. Did you try the one with the white sauce?").
		OK()
	bob.
		GotoInput(carolInitialPost.Links["💬 Reply"], "@carol I'm with Carol on this one. It was way too salty.").
		OK()
	daveReplyToCarol := dave.
		GotoInput(carolInitialPost.Links["💬 Reply"], "@carol Wait, which one? The one on 3rd or the one near the park?").
		OK()
	cl.Settle(t)

	carolReplyToDave := carol.
		GotoInput(daveReplyToCarol.Links["💬 Reply"], "@dave The one near the park. Avoid the 'special' toppings!").
		OK()
	cl.Settle(t)

	aliceReplyToCarol2 := alice.
		GotoInput(carolReplyToDave.Links["💬 Reply"], "@carol But the truffle oil there is tech-bro magic! 🧪").
		OK()
	cl.Settle(t)

	bob.
		GotoInput(aliceReplyToCarol2.Links["💬 Reply"], "@alice Truffle oil is just fake flavoring, Alice. Get real! It's all about the quality of the dough.").
		OK()
	alice.
		GotoInput(aliceReplyToCarol2.Links["💬 Reply"], "@bob It's about the molecular gastronomy, Bob! The way the volatiles interact with the cheese is fascinating. 🤓").
		OK()
	ivan.
		GotoInput(aliceReplyToCarol2.Links["💬 Reply"], "@alice I've got a sensor that measures the exact volatile organic compounds in truffle oil. Want to borrow it?").
		OK()

	frankPost := frank.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "Unpopular opinion: Tomato sauce is just filler. White sauce is where the real flavor is! 🥛🍕").
		OK()
	cl.Settle(t)

	bobReplyToFrank := bob.
		GotoInput(frankPost.Links["💬 Reply"], "@frank White sauce? That's not pizza, that's just soggy bread with alfredo. 0/10.").
		OK()
	cl.Settle(t)
	grace.
		GotoInput(frankPost.Links["💬 Reply"], "@frank It's all about the balance, Frank. A garlic-infused white base can actually highlight the toppings better.").
		OK()

	ivan.
		GotoInput(frankPost.Links["💬 Reply"], "@frank White sauce is old news. Have you tried the charcoal-infused sourdough base?").
		OK()
	frankReplyToBob := frank.
		GotoInput(bobReplyToFrank.Links["💬 Reply"], "@bob Bob, your palate is as dated as your dial-up modem! 👴").
		OK()
	cl.Settle(t)

	bob.
		GotoInput(frankReplyToBob.Links["💬 Reply"], "@frank Dial-up? At least dial-up was reliable, unlike your taste in pizza!").
		OK()
	grace.
		GotoInput(bobReplyToFrank.Links["💬 Reply"], "@bob Bob, you should really try the white base with caramelized onions. It might change your mind.").
		OK()

	judyPost := judy.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "Big bowling tournament this Friday! 🎳 Who's in? We're going for sushi after!").
		OK()
	cl.Settle(t)

	carolReplyToJudy := carol.
		GotoInput(judyPost.Links["💬 Reply"], "@judy I'll join for bowling, but sushi after physical activity? I don't know...").
		OK()
	cl.Settle(t)
	grace.
		GotoInput(judyPost.Links["💬 Reply"], "@judy I'm in! I know a place near the lanes that has the freshest yellowtail.").
		OK()

	judyReplyToCarol := judy.
		GotoInput(carolReplyToJudy.Links["💬 Reply"], "@carol Oh come on Carol, it's just raw fish! What's the worst that could happen? 😉").
		OK()
	cl.Settle(t)
	heidi.
		GotoInput(judyPost.Links["💬 Reply"], "@judy Can I come if I just eat the edamame? 😅").
		OK()

	dave.
		GotoInput(judyReplyToCarol.Links["💬 Reply"], "@judy @carol I'm with Carol. Last time I had sushi after bowling, I couldn't hit a single pin the next day. Coincidence? I think not.").
		OK()
	judy.
		GotoInput(judyReplyToCarol.Links["💬 Reply"], "@dave Dave, that's just because you're bad at bowling! 😂 Challenge accepted! I'll see you at the lanes.").
		OK()

	evePost := eve.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "Making homemade fettuccine tonight! 🍝 There's nothing like fresh pasta. Who wants to join?").
		OK()
	cl.Settle(t)

	heidiReplyToEve := heidi.
		GotoInput(evePost.Links["💬 Reply"], "@eve I'll bring the dessert! I found this amazing cannoli place.").
		OK()
	aliceReplyToEve := alice.
		GotoInput(evePost.Links["💬 Reply"], "@eve I've optimized my pasta machine with a custom 3D-printed extruder. Can I bring it over?").
		OK()
	cl.Settle(t)

	eveReplyToAlice := eve.
		GotoInput(aliceReplyToEve.Links["💬 Reply"], "@alice Alice, only if it doesn't leave plastic bits in my sauce! 👩‍🍳").
		OK()
	cl.Settle(t)

	ivan.
		GotoInput(evePost.Links["💬 Reply"], "@eve Is it gluten-free? I'm trying this new ancient grain from Mars.").
		OK()

	frank.
		GotoInput(eveReplyToAlice.Links["💬 Reply"], "@eve What's the sauce situation? Please tell me you're not doing a plain tomato margherita.").
		OK()
	eve.
		GotoInput(heidiReplyToEve.Links["💬 Reply"], "@heidi Cannolis sound perfect! The sauce is my secret family recipe, Frank. It has a little bit of everything!").
		OK()

	heidi.
		Goto(evePost.Path).
		Follow("🔁 Share").
		OK()
	alice.
		Goto(carolInitialPost.Path).
		Follow("🔁 Share").
		OK()

	ivanPoll := ivan.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "[POLL Pineapple on pizza?] Yes, it's chemistry! | No, it's a crime!").
		OK()
	cl.Settle(t)

	alice.
		Goto(ivanPoll.Path).
		Follow("📮 Vote Yes, it's chemistry!").
		OK()
	bob.
		Goto(ivanPoll.Path).
		Follow("📮 Vote No, it's a crime!").
		OK()
	carol.
		Goto(ivanPoll.Path).
		Follow("📮 Vote No, it's a crime!").
		OK()
	cl.Settle(t)

	for d, server := range cl {
		poller := outbox.Poller{
			Domain: d,
			DB:     server.DB,
			Inbox:  server.Inbox,
		}
		if err := poller.Run(t.Context()); err != nil {
			panic(err)
		}
	}
	cl.Settle(t)

	return cl
}
