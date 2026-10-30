# Spyball

Jeu d'ambiance pour soirée (anniversaire, Noël, nouvel an…), joué en parallèle de la soirée via un petit site web local.
Chaque invité découvre en début de soirée son rôle secret, cherche discrètement ses alliés et ses ennemis pendant la soirée,
et saisit ses suppositions sur le site. À la fin, l'admin clôture la partie : les rôles sont dévoilés et les scores calculés.

## Règles du jeu (référence)

Ce jeu est l'adaptation web d'un jeu papier joué à un nouvel an (`old-game-drafts/jeux-nouvel-an`).
Il doit fonctionner pour n'importe quelle soirée et pour **6 à 24 joueurs**.

### Équipes
Dans le code on ne parle que de couleurs ; le thème leur donne un nom.

| Couleur  | Thème d'origine (« bal des nations ») |
|----------|---------------------------------------|
| `blue`   | Démocraties du monde libre (OTAN) |
| `red`    | Puissances autoritaires |

Les deux camps n'ont pas forcément autant de joueurs. Il n'y a plus de neutres (les anciens `white` et `yellow`
ont été retirés).

**Traîtres** : des traîtres se cachent parmi les joueurs, environ un tous les 4 ou 5 joueurs ; leur répartition
entre les camps dépend de la configuration (dans `12-classic` : un `blue` et un `red`). Un traître est un **membre ordinaire de son camp**
pour les alliés et les ennemis (même signe, même relations), mais il a **en plus des défis** et son score
compte pour le **camp des traîtres**, pas pour celui de sa couleur. Le thème donne un nom au traître et à leur camp
(`traitor.name` « Agent double », `traitor.team` « Agents doubles »).

### Ce que reçoit chaque joueur
- **Un personnage** (ex. « France ») et son équipe.
- **Un signe d'appel** : comportement à reproduire pendant la soirée pour être repéré par ses alliés.
  Les signes `blue` sont verbaux (tics de langage, mots qui riment avec « fromage »…),
  les signes `red` sont gestuels (clin d'œil, se gratter le nez…).
- Un traître reçoit en plus **des défis** : des actions discrètes à réussir pendant la soirée
  (« accrocher un grelot à la porte », « cacher la souris dans une chaussure »…), tirées dans la liste `challenges`
  du thème ; leur nombre est fixé par la configuration (`challenges`). Les défis sont tirés **une fois pour la partie**
  (`Game.Challenges`) : tous les traîtres ont **les mêmes**. Les joueurs ne déclarent rien : à la fin, l'admin
  **coche** sur la page admin les défis réalisés, **sans dire par qui** (`Game.SetChallengesDone`).
- **Des indices** sur d'autres joueurs : « tu reconnaîtras un bleu à son signe Y ». Un indice donne la couleur
  du joueur (badge) et son signe, mais **jamais son personnage**, qui sert de mot de passe, ni s'il est traître.
  Le graphe des indices (qui connaît le signe de qui) est défini par la configuration de partie.

Il n'y a **pas de mot de passe** entre joueurs (trop compliqué en pratique) : les joueurs se reconnaissent aux signes
et se dévoilent l'un à l'autre comme ils le souhaitent.

### Tirage
L'admin saisit la liste des joueurs, choisit le thème, puis lance le tirage. La configuration est **tirée au sort**
parmi celles prévues pour ce nombre de joueurs (et que le thème peut habiller), avec une chance proportionnelle à
leur poids (`weight`, 0 = jamais) : `config.Catalog.PickSetup`. Elle reste **secrète** jusqu'à la clôture (ni la page
admin ni les règles ne la nomment, puisqu'elle dit combien il y a de traîtres) ; la page résultats la nomme.
Les emplacements de la configuration sont ensuite attribués **aléatoirement** aux joueurs
et les signes et défis sont tirés **aléatoirement** dans les listes du thème (sans doublon).
Le tirage étant aléatoire, **l'admin peut jouer** comme les autres (contrairement à la version papier).

### Pendant la soirée
Les joueurs observent, se reconnaissent, échangent leurs infos discrètement,
et saisissent/modifient sur le site leurs suppositions : pour chaque autre joueur, **allié**, **ennemi** ou rien.
Les traîtres tentent leurs défis sans rien déclarer sur le site.
Un joueur surpris à parler du jeu peut être **dénoncé publiquement**.

### Fin de partie et score
Deux joueurs du même camp sont alliés ; deux joueurs de camps différents sont ennemis. Les traîtres ne font pas exception.

L'admin clôture la saisie. Puis, pour chaque joueur (traîtres compris) :
- +2 par allié désigné comme allié, +2 par ennemi désigné comme ennemi ;
- −2 par erreur (allié désigné comme ennemi ou ennemi désigné comme allié) ;
- +3 par joueur qui l'a désigné comme allié, −5 par joueur qui l'a désigné comme ennemi,
  **que la supposition soit juste ou non** ;
- les défis ne rapportent rien à un joueur en particulier : voir le camp des traîtres ci-dessous ;
- points attribués manuellement par l'admin pendant l'affichage des résultats : dénonciations publiques, bonus divers.

Ce barème est regroupé en tête de `internal/game/score.go` ; il pourra être revu (il doit rester isolé et facile à modifier).

Score d'un camp = somme des scores de ses membres **qui ne sont pas traîtres** (si les camps n'ont pas autant
de membres comptés, le plus nombreux est avantagé : à garder en tête en écrivant les configurations). Score du **camp des traîtres** = somme des scores des traîtres + 10 par défi réalisé (coché par l'admin une fois la partie
clôturée, quel que soit le traître qui l'a fait). Les trois camps sont classés ensemble. Les scores peuvent être négatifs.

## Pages
| Page | Accès | Contenu |
|------|-------|---------|
| Règles | public | Règles du jeu, rendues selon le thème |
| Login | public | Connexion d'un joueur (voir ci-dessous) |
| Mon rôle | joueur connecté, partie en cours | Personnage, équipe, signe, indices ; saisie/modification des suppositions ; pour un traître, ses défis (sans rien à déclarer) |
| Résultats | tout le monde, **uniquement après clôture** | Rôles dévoilés, suppositions de chacun, scores par camp (traîtres compris) et par joueur ; défis des traîtres en lecture seule ; l'admin y ajoute des points (dénonciations) |
| Admin | admin | Liste des joueurs, choix du thème (la configuration est tirée au sort, voir les chances), tirage, clôture de la partie ; après la clôture, cases à cocher des défis réalisés |

**Authentification des joueurs** :
- première connexion : le joueur donne son nom (choisi dans la liste saisie par l'admin) ; le site lui révèle le nom de son personnage ;
- connexions suivantes : nom + nom du personnage. Le personnage étant secret, il sert de mot de passe.
  La saisie est tolérante (`config.NormalizeName`) : majuscules, accents, cédilles, espaces et ponctuation sont ignorés
  (« ong P.lS » vaut « ONG P.L.S. »). Les noms de joueurs et de personnages doivent donc rester distincts une fois normalisés ;
- session par cookie ; un joueur déconnecté ne peut plus voir son rôle. La session n'est valable que tant que le joueur
  garde ce personnage : annuler le tirage ou commencer une nouvelle partie déconnecte tout le monde ;
- un joueur qui a oublié son personnage demande à l'admin de « réinitialiser » sa connexion (il se reconnecte alors
  avec son seul nom). La page admin indique seulement qui s'est déjà connecté, jamais les rôles.

**Règles** (`/rules`, lien dans l'en-tête de chaque page) : texte générique dans `rules.*` de la locale, habillé par
le thème (récit `story`, noms et descriptions des camps, nom des traîtres). Thème affiché : `?theme=<id>`, sinon
celui de la partie, sinon le premier. Un thème peut remplacer n'importe quel texte de la locale par le sien (`texts:` du thème,
`Theme.Text`) : `t` et `tf` lisent les textes du thème de la partie (ou du thème affiché sur la page Règles).
`**mot**` s'affiche en gras et les retours à la ligne sont gardés là où le template passe le texte à `rich`.

**Résultats** (`/results`) : visibles par tous une fois la partie clôturée (avant, la page dit seulement d'attendre).
Classement des camps (gagnant mis en avant ; les couleurs sans leurs traîtres, puis le camp des traîtres), classement de tous les joueurs
avec leur personnage et le détail de leurs points et suppositions (✓/✗), défis des traîtres, liste des points manuels. L'admin connecté y ajoute ou annule des points
(entre −20 et +20 par entrée, avec une raison).

**L'admin pouvant jouer**, la page admin ne doit jamais afficher l'attribution des rôles, les signes, les défis
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
themes/<id>/theme.yaml   un thème : noms des camps, personnages, listes de signes et défis (format commenté dans le fichier)
setups/<id>.yaml         une configuration de partie : emplacements, traîtres, nombre de défis, graphe des indices
locales/<lang>.yaml      textes de l'interface (fr.yaml d'abord)
content.go               package racine qui embarque themes/, setups/ et locales/
old-game-drafts/    brouillons historiques, ignorés par git, simple source d'inspiration
```

Séparation importante :
- une **configuration** (`setups/`) est indépendante du thème : elle parle de `blue-1`, `red-2`…,
  marque les traîtres et décrit les indices (qui connaît le signe de qui) ;
- une configuration correspond à **un nombre exact de joueurs** : pour couvrir plusieurs tailles, on duplique
  la configuration plutôt que de gérer des rôles optionnels ;
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
Le [README](README.md) décrit l'usage (lancement, déroulé d'une soirée, ajout de thème / configuration / langue).
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
reçoit une copie de son emplacement, personnage, signes, défis et indices : la partie reste cohérente même si les YAML changent.
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
- tests HTTP dans `*_test.go` avec `newTestClient` (cookies + redirections, comme un navigateur) ;
  `e2e_test.go` joue une partie complète uniquement par HTTP, vérifie les scores avec un calcul indépendant,
  puis redémarre le serveur sur les mêmes données. À garder vert à chaque évolution des règles ;
- au démarrage, le serveur affiche les adresses du réseau local à donner aux invités.
- l'interface tutoie les joueurs : les textes des thèmes (descriptions de personnages…) aussi.

Persistance (`internal/store`) : la partie est sauvegardée dans `<-data>/game.json` (par défaut `data/game.json`,
ignoré par git). Toujours passer par `Store.Read` / `Store.Update` : `Update` travaille sur une copie, l'enregistre
(fichier temporaire puis renommage) et ne la rend visible que si tout a réussi. Un fichier illisible bloque le démarrage
au lieu d'être écrasé ; supprimer `data/game.json` repart d'une partie vierge.

Correspondance thème ↔ configuration : l'emplacement `<couleur>-N` reçoit le N-ième personnage de sa couleur
(qu'il soit traître ou non).
Dans les signes et les défis, `{blue}` et `{red}` sont remplacés par le `member` du camp correspondant.

## Feuille de route
1. **V1** : thème « bal des nations » et une configuration à **12 joueurs**
   (4 `blue`, 4 `red`, 2 `white`, ONG, Anonymous, d'après `12/11/10-44211-0` des brouillons) ; toutes les pages ci-dessus.
   Depuis, les neutres (`white`, `yellow`) ont été remplacés par des traîtres dans chaque camp :
   `setups/12-classic.yaml` a deux camps de 6 avec un traître chacun. Configurations existantes : 8, 9, 10, 11 (deux), 12 et 13 joueurs.
2. Configurations équilibrées pour tout nombre de joueurs de 6 à 24 (éventuellement générées), plusieurs par
   taille avec des poids pour varier les parties. Les camps peuvent être inégaux (nombres impairs de joueurs).
3. Autres thèmes, autres langues.

### Étapes de la V1
Chaque étape est livrée testée, dans un commit à part.
- [x] 1. Squelette : module Go, serveur HTTP, layout HTML commun, CSS de base, templates et assets embarqués.
- [x] 2. Formats YAML et chargement validé : thème « bal des nations », configuration 12 joueurs, `locales/fr.yaml`.
- [x] 3. Logique de jeu (`internal/game`) : état de la partie, tirage aléatoire, relations allié/ennemi, calcul des scores.
- [x] 4. Persistance de l'état sur disque (JSON, écriture atomique, accès concurrent protégé).
- [x] 5. Page admin : connexion admin, liste des joueurs, choix thème/configuration, tirage, clôture.
- [x] 6. Login joueur et page « Mon rôle » : rôle, signe, indices, suppositions, choix de camp des `white`.
- [x] 7. Page résultats : rôles dévoilés, suppositions, scores, ajout de points manuels par l'admin.
- [x] 8. Page règles, rendue selon le thème.
- [x] 9. Finitions : affichage mobile, test de bout en bout d'une partie complète, README (lancement, ajout d'un thème).
