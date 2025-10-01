package routes

import (
	"telo/controllers/agent"
	"telo/controllers/callback/live_casino/evolutionlive"
	"telo/controllers/callback/slots/evolutionslot"
	"telo/controllers/callback/slots/fastspin"
	"telo/controllers/callback/slots/playstar"
	"telo/controllers/callback/slots/pragmatic"
	"telo/controllers/callback/slots/spadegaming"
	"telo/controllers/callback/slots/telo"
	"telo/controllers/callback/sportsbook/sbo"
	"telo/controllers/user"
	"telo/middlewares"

	"github.com/gofiber/fiber/v2"
)

func Setup(app *fiber.App) {
	userroutes := app.Group("/user", middlewares.UserAuthMiddleware)
	userroutes.Post("/balance", user.CheckUserBalance)
	userroutes.Post("/register", user.RegisterUser)
	userroutes.Post("/transfer", user.TransferBalance)
	userroutes.Post("/games/start", user.LaunchGameHandler)

	app.Post("/agent/info", agent.AgentInfo)
	agentroutes := app.Group("/agent", middlewares.AgentAuth())
	agentroutes.Post("/register", agent.RegisterAgent)
	agentroutes.Post("/topup", agent.TopupAgentBalance)

	//providers
	teloroutes := app.Group("/seamless/slot/gold_api", middlewares.TeloAgentAuth())
	teloroutes.Post("/user_balance", telo.CheckUserBalance)
	teloroutes.Post("/game_callback", telo.ProcessSlotTransaction)

	app.Post("/seamless/sportsbook/sbo/CreateAgent", sbo.CreateAgent)
	app.Post("/seamless/sportsbook/sbo/CreateUser", sbo.CreateUser)

	//sbo
	sboroutes := app.Group("/seamless/sportsbook/sbo", middlewares.SboAuth())
	sboroutes.Post("/GetBalance", sbo.GetMemberBalanceHandler)
	sboroutes.Post("/GetBetStatus", sbo.GetBetStatusHandler)
	sboroutes.Post("/Deduct", sbo.DeductHandler)
	sboroutes.Post("/Settle", sbo.SettleHandler)
	sboroutes.Post("/Cancel", sbo.CancelBetHandler)
	sboroutes.Post("/Rollback", sbo.RollbackBetHandler)
	sboroutes.Post("/Bonus", sbo.BonusCreditHandler)

	//evolutionslot
	evo := app.Group("/seamless/live-slot/evolution", middlewares.CheckEvolutionToken())
	evo.Post("/check", evolutionslot.BalanceHandler)
	evo.Post("/balance", evolutionslot.BalanceHandler)
	evo.Post("/debit", evolutionslot.DebitHandler)
	evo.Post("/credit", evolutionslot.CreditHandler)
	evo.Post("/cancel", evolutionslot.CancelHandler)
	evo.Post("/sid", evolutionslot.UserHandler)

	//evolutionlive
	evolive := app.Group("/seamless/live-casino/evolution", middlewares.CheckEvolutionTokenLive())
	evolive.Post("/check", evolutionlive.BalanceHandler)
	evolive.Post("/balance", evolutionlive.BalanceHandler)
	evolive.Post("/debit", evolutionlive.DebitHandler)
	evolive.Post("/credit", evolutionlive.CreditHandler)
	evolive.Post("/cancel", evolutionlive.CancelHandler)
	evolive.Post("/sid", evolutionlive.UserHandler)

	//fs
	app.Post("/seamless/slot/fastspin", fastspin.GatewayHandler)
	app.Post("/seamless/slot/spadegaming", spadegaming.GatewayHandler)

	//playstar
	psroutes := app.Group("/seamless/slot/api")
	psroutes.Get("/bet", playstar.BetHandler)
	psroutes.Get("/result", playstar.ResultHandler)
	psroutes.Get("/refund", playstar.RefundHandler)
	psroutes.Get("/bonusaward", playstar.BonusAwardHandler)
	psroutes.Get("/getbalance", playstar.GetBalanceHandler)

	//pragmatic
	prroutes := app.Group("/seamless/provider/pragmatic/")
	prroutes.Post("/authenticate", pragmatic.AuthenticateHandler)
	prroutes.Post("/balance", pragmatic.Balance)
	prroutes.Post("/bet", pragmatic.Bet)
	prroutes.Post("/bonuswin", pragmatic.BonusWin)
	prroutes.Post("/endround", pragmatic.EndRound)
	prroutes.Post("/jackpotwin", pragmatic.JackpotWin)
	prroutes.Post("/promowin", pragmatic.PromoWin)
	prroutes.Post("/refund", pragmatic.Refund)
	prroutes.Post("/result", pragmatic.Result)
	prroutes.Post("/adjustment", pragmatic.Adjustment)
}
