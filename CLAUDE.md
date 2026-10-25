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
- **Un mot de passe** : un autre joueur qui le connaît peut le lui donner pour prouver son identité.
- **Des indices** sur d'autres joueurs : « tu reconnaîtras X à son signe » et éventuellement « son mot de passe est Y ».
  Le graphe des indices (qui connaît quoi sur qui) est défini par la configuration de partie.

### Pendant la soirée
Les joueurs observent, s'identifient (en donnant le mot de passe sur un papier), échangent leurs infos discrètement,
et saisissent/modifient sur le site leurs suppositions : pour chaque autre joueur, **allié**, **ennemi** ou rien.
Un joueur surpris à parler du jeu peut être **dénoncé publiquement** (l'admin enregistre la dénonciation).

### Fin de partie et score (version de travail, à confirmer)
L'admin clôture la saisie. Les `white` choisissent leur camp. Puis, pour chaque joueur :
- +1 par allié correctement identifié, +1 par ennemi correctement identifié ;
- −1 par ennemi désigné comme allié, −1 par allié désigné comme ennemi ;
- −1 par dénonciation publique subie (les neutres gagnent +1 par dénonciation qu'ils font).

Score d'équipe = somme des scores des membres. Le score de l'équipe neutre est multiplié par `4/n` (n = nombre de neutres)
pour compenser son désavantage.

Règles spécifiques `yellow` (thème d'origine) :
- **ONG** : considérée comme alliée de tout le monde pour le décompte ; gagne/perd avec les neutres.
- **Anonymous** : considéré comme ennemi de tout le monde (sauf des neutres) ; connaît les signes de presque tous ;
  bonus s'il désigne comme ennemi les leaders `blue-1`/`red-1` après s'être fait passer pour leur allié ; −2 s'il désigne l'ONG comme ennemie.

Les règles spéciales doivent être décrites dans la config (YAML), pas codées en dur par personnage : le code connaît des
**types de règles** (ex. `ally_of_everyone`, `enemy_of_everyone`, `chooses_team`), le thème leur donne un nom et un texte.

## Pages
| Page | Accès | Contenu |
|------|-------|---------|
| Règles | public | Règles du jeu, rendues selon le thème |
| Login | public | Connexion d'un joueur |
| Mon rôle | joueur connecté, partie en cours | Personnage, équipe, signe, mot de passe, indices ; saisie/modification des suppositions ; choix de camp pour les `white` |
| Résultats | tout le monde, **uniquement après clôture** | Rôles dévoilés, suppositions de chacun, scores par joueur et par équipe |
| Admin | admin | Liste des joueurs, choix du thème et de la configuration, tirage des rôles, dénonciations, clôture de la partie |

Un joueur déconnecté ne doit plus pouvoir voir son rôle (session par cookie).
Les pages sont pensées **mobile d'abord** : les joueurs utilisent leur téléphone pendant la soirée.

## Organisation du dépôt (cible)
```
cmd/spyball/        point d'entrée (flags : port, dossier de données, mot de passe admin)
internal/game/      modèle et logique pure : rôles, tirage, suppositions, score  (sans HTTP, bien testé)
internal/config/    chargement et validation des YAML (thèmes, configurations, traductions)
internal/store/     persistance de l'état de la partie
internal/web/       handlers HTTP, sessions, rendu des templates
web/templates/      templates html/template
web/static/         CSS (et JS minimal si vraiment nécessaire)
themes/<theme>/     un dossier par thème (YAML) : noms des équipes, personnages, signes, mots de passe, règles spéciales
setups/             configurations de partie par nombre de joueurs (YAML) : rôles présents et graphe des indices
locales/            textes de l'interface (fr.yaml d'abord)
old-game-drafts/    brouillons historiques, ignorés par git, simple source d'inspiration
```

Séparation importante :
- une **configuration** (`setups/`) est indépendante du thème : elle parle de `blue-1`, `red-2`, `white-1`, `yellow-1`…
  et décrit les indices (qui connaît le signe et/ou le mot de passe de qui) ;
- un **thème** (`themes/`) associe chaque emplacement à un personnage et fournit les pools de signes et de mots de passe.
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
À compléter quand le projet sera initialisé (`go run ./cmd/spyball`, `go test ./...`).

## Feuille de route
1. **V1** : un thème (« bal des nations ») et une configuration pour un nombre de joueurs fixe ; toutes les pages ci-dessus.
2. Configurations équilibrées pour tout nombre de joueurs de 6 à 24 (éventuellement générées).
3. Autres thèmes, autres langues.

## Questions ouvertes
- Login : comment un joueur s'authentifie-t-il (nom + code généré par l'admin, lien/QR code personnel…) ?
- Le score : quelle version des règles retenir (les brouillons en contiennent plusieurs) ?
  Bonus « allié qui se reconnaît mutuellement » ? Dénonciations : saisies par l'admin ?
- Indices : distinguer « signe seulement » et « signe + mot de passe » dans les configurations ?
- Nom des configurations des brouillons (`12/11/10-44211-0`) : une configuration couvre-t-elle plusieurs tailles de groupe
  (rôles optionnels) ?
- Le tirage des rôles est-il aléatoire, et l'organisateur peut-il jouer (rôle neutre tiré au hasard) ?
