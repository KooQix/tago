# TaGo 🏷️

Custom Struct Tag Parser for Go

**TaGo** is a lightweight Go library that makes working with **custom struct tags** simple.\
It allows you to define, extract, and apply tags to your models — with support for **nested structs**.

This is especially useful if you:

-   Want ORM-like behavior without heavy abstractions

-   Need to define domain-specific struct tags (custom serialization, validation, masking, preloading, etc.)

-   Work with **GORM**, but want more control over how custom tags are applied

---

## ✨ Features

-   Parse **any** struct tag with a given name (e.g. `gorm2`, `validate`, `mask`).

-   Extract **instructions** in key=value format.

-   Traverse nested structs with **prefixes** for field names.

-   Easy to **apply** instructions to your own logic.

-   **Performance-friendly if cached**: Compute instructions **once per model** (via `Get` / `GetNested`) and reuse them with `Apply` / `ApplyOne`.

-   Zero dependencies (only `reflect` & `strings`).

---

## 📦 Installation

```bash
go get github.com/KooQix/tago
```

---

## ⚠️ Performance Note

Using Go’s `reflect` and iterating over struct fields can be **costly** if done repeatedly inside hot loops or frequently called endpoints.\
**Best practice:**

1. **Compute once** — Call `Get` or `GetNested` once per model type.

2. **Cache in a variable** or attach it to your repository/service layer.

3. **Reuse** the cached `Instructions` with `Apply` / `ApplyOne`.

This way, you pay the reflection cost only once rather than for every query.

---

## 🔧 Basic Example

```go
type MyModel struct {
    Field1 string `gorm2:"preload=true;otherOption=custom"`
}

t := tago.TaGo{Name: "gorm2"}

// ✅ Compute once and cache
cachedTags := t.Get(&MyModel{})

// Use tags later without reflection overhead
t.Apply(cachedTags, map[tago.Instruction]func(f tago.FieldName){
    tago.Instruction("preload=true"): func(f tago.FieldName) { fmt.Println("Preloading", f) },
})
```

---

## ⚡ Usage with GORM

Preloading relations is a common use case, and preloading nested structs can be tedious (especially nested ones).\
If you want to **automatically preload relations** based on struct tags, TaGo can help.

Let’s say you have two entities: `User` and `Address`.\
You want to automatically **preload** addresses whenever querying users.

### User model (with custom tag)

```go
type User struct {
	ID        uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" gorm:"not null"`
	Email     string `json:"email" gorm:"unique;not null"`

	AddressID uint64
	Address   Address `json:"address" gorm:"not null;foreignKey:AddressID;references:ID" gorm2:"preload=true"`
}
```

### Address model

```go
type Address struct {
	ID      uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	Street  string `json:"street" gorm:"not null"`
	City    string `json:"city" gorm:"not null"`
}
```

### GORM2 wrapper

```go
var gorm2Tag = tago.TaGo{Name: "gorm2"}

type Gorm2 struct {
	*gorm.DB
	tags tago.Instructions
}

func Create(db *gorm.DB, model interface{}) *Gorm2 {
	// ✅ Compute tags *once* for the model
	return &Gorm2{
		DB:   db,
		tags: gorm2Tag.GetNested(model, "."), // Use "." as separator for nested fields
	}
}

func (db *Gorm2) Preloads() *gorm.DB {
	query := db.DB
	gorm2Tag.ApplyOne(tago.Instruction("preload=true"), db.tags, func(field tago.FieldName) {
		query = query.Preload(field.String())
	})
	return query
}
```

Now, when you query for a **User**, the `Address` relation will be **auto-preloaded** because of your custom `gorm2:"preload=true"` tag.

---

## 🔍 Another Example: Validation with `validate`

You can implement your own validation rules with tags.

```go
type Product struct {
	Name  string `validate:"required"`
	Stock int    `validate:"min=1"`
}

// ✅ Cache instructions once
var validateTags = tago.TaGo{Name: "validate"}.Get(Product{})

func Validate(p Product) error {
	t := tago.TaGo{Name: "validate"}

	var err error

	// Recover from panics to return as error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("validation error: %v", r)
		}
	}()

	t.ApplyOne(tago.Instruction("required"), validateTags, func(field tago.FieldName) {
		val := reflect.ValueOf(p).FieldByName(field.String())
		if val.Kind() == reflect.String && val.String() == "" {
			panic(fmt.Sprintf("%s is required", field))
		}
	})

	t.ApplyOne(tago.Instruction("min=1"), validateTags, func(field tago.FieldName) {
		val := reflect.ValueOf(p).FieldByName(field.String())
		if val.Kind() == reflect.Int && val.Int() < 1 {
			panic(fmt.Sprintf("%s must be >= 1", field))
		}
	})
	return err
}
```

---

## 🔒 Example: Masking Sensitive Fields

```go
type Customer struct {
	Name     string `json:"name"`
	Email    string `mask:"email"`
	Password string `mask:"!!!"`
}

// ✅ Cache mask tags
var maskTags = tago.TaGo{Name: "mask"}.Get(Customer{})

func Mask(c *Customer) {
	t := tago.TaGo{Name: "mask"}
	t.Apply(maskTags, map[tago.Instruction]func(f tago.FieldName){
		"email": func(f tago.FieldName) {
			v := reflect.ValueOf(c).Elem().FieldByName(f.String())
			v.SetString("hidden@email.com")
		},
		"!!!": func(f tago.FieldName) {
			v := reflect.ValueOf(c).Elem().FieldByName(f.String())
			v.SetString("***")
		},
	})
}
```

---

## 🤝 Contributing

PRs are welcome! 🎉\
Have ideas for new examples (custom tags like `serialize`, `encrypt`, `index`)? Feel free to open an issue or submit a pull request.

---

## 📜 License

MIT License. Free to use and modify.
