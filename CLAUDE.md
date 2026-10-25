# Spyball

Jeu d'ambiance pour soirée (anniversaire, Noël, nouvel an…), joué en parallèle de la soirée via un petit site web local.
Chaque invité découvre en début de soirée son rôle secret, cherche discrètement ses alliés et ses ennemis pendant la soirée,
et saisit ses suppositions sur le site. À la fin, l'admin clôture la partie : les rôles sont dévoilés et les scores calculés.

## Règles du jeu (référence)

Ce jeu est l'adaptation web d'un jeu papier joué à un nouvel an (`old-game-drafts/jeux-nouvel-an`).
Il doit fonctionner pour n'importe quelle soirée et pour **6 à 24 joueurs**.

### Équipes
Dans le code on ne parle que de couleurs ; le thème leur donne un nom.

| Couleur  | Rôle dans le jeu | Thème d'origine (« bal des nations ») |
|----------|------------------|---------------------------------------|
| `blue`   | Équipe classique | Démocraties du monde libre (OTAN) |
| `red`    | Équipe classique | Puissances autoritaires |
| `white`  | Neutre qui **choisit** son camp en fin de partie : `blue`, `red` ou rester neutre | États non alignés (Inde, Brésil…) |
| `yellow` | Neutre au rôle **asymétrique** avec des règles de score spécifiques | ONG organisatrice (alliée de tous), Anonymous (ennemi de tous) |

Il y a autant de `blue` que de `red`. Les neutres (`white` + `yellow`) sont la variable d'ajustement.
Les neutres qui restent neutres forment ensemble une équipe pour le score.

### Ce que reçoit chaque joueur
- **Un personnage** (ex. « France ») et son équipe.
- **Un signe d'appel** : comportement à reproduire pendant la soirée pour être repéré par ses alliés.
  Les signes `blue` sont verbaux (tics de langage, mots qui riment avec « fromage »…),
  les signes `red` sont gestuels (clin d'œil, se gratter le nez…),
  les signes `yellow` sont des défis (« faire chanter un rouge », lancer un concours de blagues…).
  Un `white` n'a pas de liste à lui : il tire **un signe bleu et un signe rouge**, et choisit ainsi
  par quel camp il veut se faire reconnaître.
- **Des indices** sur d'autres joueurs : « tu reconnaîtras le personnage X à son signe Y ».
  Le graphe des indices (qui connaît le signe de qui) est défini par la configuration de partie.
  Un indice sur un `white` donne son signe bleu à un `blue`, son signe rouge à un `red`, et ses deux signes
  à un `white` ou un `yellow`.

Il n'y a **pas de mot de passe** entre joueurs (trop compliqué en pratique) : les joueurs se reconnaissent aux signes
et se dévoilent l'un à l'autre comme ils le souhaitent.

### Tirage
L'admin saisit la liste des joueurs, choisit le thème et la configuration correspondant au nombre de joueurs,
puis lance le tirage : les emplacements de la configuration sont attribués **aléatoirement** aux joueurs
et les signes sont tirés **aléatoirement** dans les listes du thème (sans doublon).
Le tirage étant aléatoire, **l'admin peut jouer** comme les autres (contrairement à la version papier).

### Pendant la soirée
Les joueurs observent, se reconnaissent, échangent leurs infos discrètement,
et saisissent/modifient sur le site leurs suppositions : pour chaque autre joueur, **allié**, **ennemi** ou rien.
Un joueur surpris à parler du jeu peut être **dénoncé publiquement**.

### Fin de partie et score
Un `white` choisit son camp (`blue`, `red` ou neutre) sur sa page et peut changer d'avis jusqu'à la clôture.
Pour le score, il est alors **entièrement** considéré comme membre du camp choisi.

Qui est allié de qui :
- deux joueurs du même camp sont alliés ; deux joueurs de camps différents sont ennemis ;
- un neutre resté neutre est donc un **ennemi** pour les `blue` et les `red`, et un allié pour les autres neutres ;
- exceptions `yellow` : l'ONG est l'alliée de tout le monde, l'Anonymous est l'ennemi de tout le monde ;
  entre les deux, « allié de tous » l'emporte (l'ONG et l'Anonymous sont alliés). La relation est toujours symétrique.

L'admin clôture la saisie. Puis, pour chaque joueur :
- +1 par allié correctement identifié, +1 par ennemi correctement identifié ;
- −1 par ennemi désigné comme allié, −1 par allié désigné comme ennemi ;
- points attribués manuellement par l'admin pendant l'affichage des résultats :
  défis réussis par les `yellow`, dénonciations publiques (−1 pour le dénoncé, +1 pour un neutre qui dénonce).

Ce barème est celui de la V1 ; il pourra être revu ensuite (il doit donc rester isolé et facile à modifier).

Score d'équipe = somme des scores des membres. Le score de l'équipe neutre est multiplié par `4/n` (n = nombre de neutres)
pour compenser son désavantage.

Règles spécifiques `yellow` (thème d'origine) :
- **ONG** : alliée de tout le monde pour le décompte ; son score compte dans l'équipe neutre.
- **Anonymous** : ennemi de tout le monde pour le décompte ; son score compte dans l'équipe neutre ;
  il connaît les signes de beaucoup de joueurs (via les indices de la configuration).
  Ses bonus éventuels (piéger un leader, etc.) sont attribués manuellement par l'admin en fin de partie.

Les règles spéciales doivent être décrites dans la config (YAML), pas codées en dur par personnage : le code connaît des
**types de règles** (ex. `ally_of_everyone`, `enemy_of_everyone`, `chooses_team`), le thème leur donne un nom et un texte.

## Pages
| Page | Accès | Contenu |
|------|-------|---------|
| Règles | public | Règles du jeu, rendues selon le thème |
| Login | public | Connexion d'un joueur (voir ci-dessous) |
| Mon rôle | joueur connecté, partie en cours | Personnage, équipe, signe, indices ; saisie/modification des suppositions ; choix de camp pour les `white` |
| Résultats | tout le monde, **uniquement après clôture** | Rôles dévoilés, suppositions de chacun, scores par joueur et par équipe ; l'admin peut y ajouter des points (défis, dénonciations) |
| Admin | admin | Liste des joueurs, choix du thème et de la configuration, tirage, clôture de la partie |

**Authentification des joueurs** :
- première connexion : le joueur donne son nom (choisi dans la liste saisie par l'admin) ; le site lui révèle le nom de son personnage ;
- connexions suivantes : nom + nom du personnage. Le personnage étant secret, il sert de mot de passe.
  La saisie est tolérante (`config.NormalizeName`) : majuscules, accents, cédilles, espaces et ponctuation sont ignorés
  (« ong P.lS » vaut « ONG P.L.S. »). Les noms de joueurs et de personnages doivent donc rester distincts une fois normalisés ;
- session par cookie ; un joueur déconnecté ne peut plus voir son rôle. La session n'est valable que tant que le joueur
  garde ce personnage : annuler le tirage ou commencer une nouvelle partie déconnecte tout le monde ;
- un joueur qui a oublié son personnage demande à l'admin de « réinitialiser » sa connexion (il se reconnecte alors
  avec son seul nom). La page admin indique seulement qui s'est déjà connecté, jamais les rôles.

**L'admin pouvant jouer**, la page admin ne doit jamais afficher l'attribution des rôles, les signes
ni les suppositions avant la clôture. L'admin est authentifié séparément (mot de passe passé au lancement du serveur).
Les pages sont pensées **mobile d'abord** : les joueurs utilisent leur téléphone pendant la soirée.

## Organisation du dépôt (cible)
```
cmd/spyball/        point d'entrée (flags : port, dossier de données, mot de passe admin)
internal/game/      modèle et logique pure : rôles, tirage, suppositions, score  (sans HTTP, bien testé)
internal/config/    chargement et validation des YAML (thèmes, configurations, traductions)
internal/store/     persistance de l'état de la partie
internal/server/    handlers HTTP, sessions, rendu des templates
web/                package Go qui embarque (embed) les templates et les assets
web/templates/      templates html/template : layout.html + une page par fichier dans pages/
web/static/         CSS (et JS minimal si vraiment nécessaire)
themes/<id>/theme.yaml   un thème : noms des camps, personnages, listes de signes (format commenté dans le fichier)
setups/<id>.yaml         une configuration de partie : emplacements, règles jaunes, graphe des indices
locales/<lang>.yaml      textes de l'interface (fr.yaml d'abord)
content.go               package racine qui embarque themes/, setups/ et locales/
old-game-drafts/    brouillons historiques, ignorés par git, simple source d'inspiration
```

Séparation importante :
- une **configuration** (`setups/`) est indépendante du thème : elle parle de `blue-1`, `red-2`, `white-1`, `yellow-1`…
  et décrit les indices (qui connaît le signe de qui) ;
- une configuration correspond à **un nombre exact de joueurs** : pour couvrir plusieurs tailles, on duplique
  la configuration en retirant des neutres, plutôt que de gérer des rôles optionnels ;
- un **thème** (`themes/`) associe chaque emplacement à un personnage et fournit les listes de signes par couleur.
  Ajouter un thème (Noël : lutins vs disciples du père Fouettard ; Halloween : sorciers vs fantômes) ne doit demander
  **aucune modification de code**.

## Précisions techniques
- Go (dernière version stable), bibliothèque standard en priorité (`net/http`, `html/template`, `embed`).
  Dépendances externes seulement si justifiées (ex. un parseur YAML).
- Pas de framework front : HTML rendu côté serveur + CSS. JavaScript seulement pour du confort.
- Un seul binaire, templates et assets embarqués ; tourne sur le réseau local de la soirée.
- Une seule partie à la fois. L'état est persisté sur disque (fichier JSON) pour survivre à un redémarrage du serveur.
- Code, identifiants, commentaires et messages de commit en **anglais**. Interface en **français** via `locales/`,
  sans texte en dur dans les templates, pour pouvoir ajouter d'autres langues plus tard.
- La logique de jeu (`internal/game`) est couverte par des tests unitaires, en particulier le calcul des scores.

## Commandes
```bash
go run ./cmd/spyball -addr :8080   # lance le serveur sur http://localhost:8080
SPYBALL_ADMIN_PASSWORD=xxx go run ./cmd/spyball   # mot de passe admin (sinon il est généré et affiché au démarrage)
go test ./...                      # tests
go vet ./...
go run ./cmd/spyball -content .   # lit thèmes/configurations/traductions sur disque au lieu du binaire
go test -race ./...                # tests avec détection des accès concurrents
```
Le contenu YAML est validé au démarrage (champs inconnus refusés, cohérence des emplacements et des indices,
noms de personnages uniques…) : le serveur refuse de démarrer et liste toutes les erreurs.
Chaque page est un fichier `web/templates/pages/<nom>.html` définissant les blocs `title` et `content`,
rendu dans `layout.html` via `Server.render(w, status, "<nom>", data)`.
Tout texte d'interface passe par `{{t "cle.sous_cle"}}` ; une clé absente de la locale fait échouer le rendu.

Logique de jeu (`internal/game`) : `Game` est un état autonome sérialisable en JSON. Au tirage, chaque joueur
reçoit une copie de son emplacement, personnage, signes et indices : la partie reste cohérente même si les YAML changent.
Phases `setup` → `running` → `closed` (`CancelDraw` permet de revenir à `setup` pour corriger la liste des joueurs).
Le barème est regroupé en tête de `internal/game/score.go`. Le tirage reçoit un `*rand.Rand` (reproductible en test).
`Game` n'est pas protégé contre les accès concurrents : c'est le rôle de l'appelant.

Serveur web (`internal/server`) :
- formulaires HTML classiques en POST, puis redirection (303) ; en cas d'erreur la page est ré-affichée avec le message.
  Les erreurs métier de `internal/game` sont des sentinelles (`game.ErrDuplicateName`…) traduites via `errors.*` de la locale
  (`internal/server/errors.go`) ;
- sessions dans des cookies signés HMAC (`session.go`), clé conservée dans `<-data>/session.key` pour survivre aux redémarrages ;
  cookies admin et joueur séparés (l'admin peut jouer depuis le même téléphone) ;
- protection CSRF par `http.CrossOriginProtection` ; pages servies en `Cache-Control: no-store` ;
- tests HTTP de bout en bout dans `*_test.go` avec `newTestClient` (cookies + redirections, comme un navigateur).
- l'interface tutoie les joueurs : les textes des thèmes (descriptions de personnages…) aussi.

Persistance (`internal/store`) : la partie est sauvegardée dans `<-data>/game.json` (par défaut `data/game.json`,
ignoré par git). Toujours passer par `Store.Read` / `Store.Update` : `Update` travaille sur une copie, l'enregistre
(fichier temporaire puis renommage) et ne la rend visible que si tout a réussi. Un fichier illisible bloque le démarrage
au lieu d'être écrasé ; supprimer `data/game.json` repart d'une partie vierge.

Correspondance thème ↔ configuration : l'emplacement `<couleur>-N` reçoit le N-ième personnage de sa couleur ;
un emplacement jaune reçoit le premier personnage jaune libre ayant la même `rule`.
Dans les signes, `{blue}`, `{red}` et `{neutral}` sont remplacés par le `member` du camp correspondant.

## Feuille de route
1. **V1** : thème « bal des nations » et une configuration à **12 joueurs**
   (4 `blue`, 4 `red`, 2 `white`, ONG, Anonymous, d'après `12/11/10-44211-0` des brouillons) ; toutes les pages ci-dessus.
2. Configurations équilibrées pour tout nombre de joueurs de 6 à 24 (éventuellement générées).
3. Autres thèmes, autres langues.

### Étapes de la V1
Chaque étape est livrée testée, dans un commit à part.
- [x] 1. Squelette : module Go, serveur HTTP, layout HTML commun, CSS de base, templates et assets embarqués.
- [x] 2. Formats YAML et chargement validé : thème « bal des nations », configuration 12 joueurs, `locales/fr.yaml`.
- [x] 3. Logique de jeu (`internal/game`) : état de la partie, tirage aléatoire, relations allié/ennemi, calcul des scores.
- [x] 4. Persistance de l'état sur disque (JSON, écriture atomique, accès concurrent protégé).
- [x] 5. Page admin : connexion admin, liste des joueurs, choix thème/configuration, tirage, clôture.
- [x] 6. Login joueur et page « Mon rôle » : rôle, signe, indices, suppositions, choix de camp des `white`.
- [ ] 7. Page résultats : rôles dévoilés, suppositions, scores, ajout de points manuels par l'admin.
- [ ] 8. Page règles, rendue selon le thème.
- [ ] 9. Finitions : affichage mobile, test de bout en bout d'une partie complète, README (lancement, ajout d'un thème).
