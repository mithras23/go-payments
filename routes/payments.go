package routes

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"
    "math"

    "github.com/go-chi/chi"
    "github.com/google/uuid"
    "grid/go-payments/db"
    "grid/go-payments/middleware"
    "grid/go-payments/models"
)

func PaymentsRoutes() *chi.Mux {
    router := chi.NewRouter()
    router.Post("/", CreatePayment)
    router.With(paginate).Get("/", GetPayments)
    router.Delete("/", DeleteAll)
    router.Get("/count", GetCount)
    router.Get("/{paymentId}", GetAPayment)
    router.Get("/{payments/stats}", GetPaymentStats)
    router.Put("/{paymentId}", CreatePaymentWithId)
    return router
}

// https://blog.philipphauer.de/web-api-pagination-timestamp-id-continuation-token/
func paginate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        pageSizeStr := r.URL.Query().Get("pageSize")
        continuationTokenStr := r.URL.Query().Get("continuationToken")

        var pageSize, pageSizeErr = strconv.Atoi(pageSizeStr)

        if pageSizeErr != nil {
            pageSize = 10
        }

        continuationToken := models.TokenFromString(continuationTokenStr)

        getPaymentsSince(continuationToken, pageSize, w, r)
    })
}

func getPaymentsSince(token *models.ContinuationToken, pageSize int, w http.ResponseWriter, r *http.Request) {
    var payments []models.PaymentDto

    var count int
    db.DB.Find(&payments).Count(&count)

    query := db.DB.Limit(pageSize)

    if token != nil {
        query = db.DB.
            Limit(pageSize).
            Where("(date_occurred > ?", token.Timestamp).
            Or("date_occurred = ? AND id > ?) AND date_occurred < ?", token.Timestamp, token.Id, time.Now())
    }

    query.Order("date_occurred asc").Order("id asc").Find(&payments)

    page := models.PageDTO{TotalCount: count, HasNext: false, Payments: payments}

    if len(payments) == pageSize {
        nextToken := models.TokenFromPaymentDto(payments[pageSize-1])
        page = models.PageDTO{TotalCount: count, HasNext: true, ContinuationToken: nextToken, Payments: payments,
        NextPageUrl: models.UrlParamFromToken(nextToken, r.URL)}
    }

    middleware.RespondWithJSON(w, http.StatusOK, &page)
}

func CreatePayment(w http.ResponseWriter, r *http.Request) {
    var paymentDto models.PaymentDto
    json.NewDecoder(r.Body).Decode(&paymentDto)
    paymentDto.Id = uuid.New()
    db.DB.Save(&paymentDto)

    middleware.RespondCreatedWithLocationHeader(w, r.URL.String(), paymentDto.Id.String())
}

func CreatePaymentWithId(w http.ResponseWriter, r *http.Request) {
    var payment models.PaymentDto
    json.NewDecoder(r.Body).Decode(&payment)
    payment.Id = uuid.MustParse(chi.URLParam(r, "paymentId"))
    db.DB.Save(&payment)

    middleware.RespondCreatedWithLocationHeader(w, r.URL.String(), payment.Id.String())
}

func GetPayments(w http.ResponseWriter, r *http.Request) {
    var payments []models.PaymentDto
    db.DB.Order("date_occurred asc").Order("id asc").Find(&payments)
    middleware.RespondWithJSON(w, http.StatusOK, &payments)
}

func GetPaymentStats(w http.ResponseWriter, r *http.Request) {
    var payments []models.PaymentDto
    db.DB.Where("date_occurred >= '2018-01-01' AND date_occurred < '2019-01-01'").Order("date_occurred asc").Order("id asc").Find(&payments)

    stats := new(models.StatsDto)
    stats.SpendingsByCatSum = make(map[models.PaymentCategory]float64, 0)
    stats.SpendingsByCatPercent = make(map[models.PaymentCategory]float64, 0)

    for _, payment := range payments {
          if "DEBT" == payment.Type {
              stats.SpendingsSum += payment.Value
              stats.SpendingsByCatSum[payment.Category] += payment.Value
              stats.SpendingsByCatPercent[payment.Category] += payment.Value
          }
    }

    for category, debtByCategorySum := range stats.SpendingsByCatPercent {
        stats.SpendingsByCatPercent[category] = math.Round( debtByCategorySum / stats.SpendingsSum * 100) / 100
    }

    for category, debtByCategorySum := range stats.SpendingsByCatSum {
        stats.SpendingsByCatSum[category] = math.Round( debtByCategorySum * 100) / 100
    }

    stats.SpendingsSum = math.Round( stats.SpendingsSum * 100) / 100

    middleware.RespondWithJSON(w, http.StatusOK, &stats)
}

func GetCount(w http.ResponseWriter, r *http.Request) {
    var count int
    db.DB.Model(&models.PaymentDto{}).Count(&count)
    middleware.RespondWithJSON(w, http.StatusOK, count)
}

func GetAPayment(w http.ResponseWriter, r *http.Request) {
    var payment models.PaymentDto
    payment.Id = uuid.MustParse(chi.URLParam(r, "paymentId"))
    db.DB.Find(&payment)
    middleware.RespondWithJSON(w, http.StatusOK, &payment)
}

func DeleteAll(w http.ResponseWriter, r *http.Request) {
    var payments []models.PaymentDto
    db.DB.Delete(payments)
}
